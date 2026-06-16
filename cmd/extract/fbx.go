package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"hsdemo/kmdata"
)

type fbxNode struct {
	name     string
	props    []any
	children []*fbxNode
}

func parseFBXGeometries(path string) ([]kmdata.MeshGeometry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) < 27 || !strings.HasPrefix(string(raw[:21]), "Kaydara FBX Binary") {
		return nil, fmt.Errorf("not a binary FBX")
	}
	p := &fbxParser{data: raw, wide: binary.LittleEndian.Uint32(raw[23:27]) >= 7500, off: 27}
	var roots []*fbxNode
	for p.off < len(raw) {
		n, ok, err := p.readNode()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		roots = append(roots, n)
	}
	var out []kmdata.MeshGeometry
	for _, root := range roots {
		walkFBX(root, func(n *fbxNode) {
			if n.name != "Geometry" {
				return
			}
			if g, ok := meshGeometryFromFBXNode(n); ok {
				out = append(out, g)
			}
		})
	}
	return out, nil
}

type fbxParser struct {
	data []byte
	wide bool
	off  int
}

func (p *fbxParser) readNode() (*fbxNode, bool, error) {
	start := p.off
	if start+p.headerLen() > len(p.data) {
		return nil, false, io.ErrUnexpectedEOF
	}
	end, props, _, nameLen := p.nodeHeader(start)
	if end == 0 && props == 0 && nameLen == 0 {
		p.off += p.headerLen()
		return nil, false, nil
	}
	p.off += p.headerLen()
	if p.off+int(nameLen) > len(p.data) {
		return nil, false, io.ErrUnexpectedEOF
	}
	name := string(p.data[p.off : p.off+int(nameLen)])
	p.off += int(nameLen)
	n := &fbxNode{name: name}
	for i := uint64(0); i < props; i++ {
		prop, err := p.readProp()
		if err != nil {
			return nil, false, fmt.Errorf("%s prop %d: %w", name, i, err)
		}
		n.props = append(n.props, prop)
	}
	for p.off < int(end) {
		child, ok, err := p.readNode()
		if err != nil {
			return nil, false, err
		}
		if !ok {
			break
		}
		n.children = append(n.children, child)
	}
	p.off = int(end)
	return n, true, nil
}

func (p *fbxParser) headerLen() int {
	if p.wide {
		return 25
	}
	return 13
}

func (p *fbxParser) nodeHeader(off int) (end, props, propLen uint64, nameLen byte) {
	if p.wide {
		return binary.LittleEndian.Uint64(p.data[off:]), binary.LittleEndian.Uint64(p.data[off+8:]),
			binary.LittleEndian.Uint64(p.data[off+16:]), p.data[off+24]
	}
	return uint64(binary.LittleEndian.Uint32(p.data[off:])), uint64(binary.LittleEndian.Uint32(p.data[off+4:])),
		uint64(binary.LittleEndian.Uint32(p.data[off+8:])), p.data[off+12]
}

func (p *fbxParser) readProp() (any, error) {
	if p.off >= len(p.data) {
		return nil, io.ErrUnexpectedEOF
	}
	code := p.data[p.off]
	p.off++
	switch code {
	case 'Y':
		v := int16(binary.LittleEndian.Uint16(p.data[p.off:]))
		p.off += 2
		return v, nil
	case 'C':
		v := p.data[p.off] != 0
		p.off++
		return v, nil
	case 'I':
		v := int32(binary.LittleEndian.Uint32(p.data[p.off:]))
		p.off += 4
		return v, nil
	case 'F':
		v := float64(math.Float32frombits(binary.LittleEndian.Uint32(p.data[p.off:])))
		p.off += 4
		return v, nil
	case 'D':
		v := math.Float64frombits(binary.LittleEndian.Uint64(p.data[p.off:]))
		p.off += 8
		return v, nil
	case 'L':
		v := int64(binary.LittleEndian.Uint64(p.data[p.off:]))
		p.off += 8
		return v, nil
	case 'S', 'R':
		n := int(binary.LittleEndian.Uint32(p.data[p.off:]))
		p.off += 4
		if p.off+n > len(p.data) {
			return nil, io.ErrUnexpectedEOF
		}
		v := p.data[p.off : p.off+n]
		p.off += n
		if code == 'R' {
			return append([]byte(nil), v...), nil
		}
		return string(v), nil
	case 'd', 'f', 'i', 'l', 'b':
		return p.readArray(code)
	default:
		return nil, fmt.Errorf("unsupported property code %q", code)
	}
}

func (p *fbxParser) readArray(code byte) (any, error) {
	if p.off+12 > len(p.data) {
		return nil, io.ErrUnexpectedEOF
	}
	n := int(binary.LittleEndian.Uint32(p.data[p.off:]))
	enc := binary.LittleEndian.Uint32(p.data[p.off+4:])
	clen := int(binary.LittleEndian.Uint32(p.data[p.off+8:]))
	p.off += 12
	if p.off+clen > len(p.data) {
		return nil, io.ErrUnexpectedEOF
	}
	payload := p.data[p.off : p.off+clen]
	p.off += clen
	if enc == 1 {
		zr, err := zlib.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		payload, err = io.ReadAll(zr)
		if err != nil {
			return nil, err
		}
	}
	switch code {
	case 'd':
		out := make([]float64, n)
		for i := range out {
			out[i] = math.Float64frombits(binary.LittleEndian.Uint64(payload[i*8:]))
		}
		return out, nil
	case 'f':
		out := make([]float64, n)
		for i := range out {
			out[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(payload[i*4:])))
		}
		return out, nil
	case 'i':
		out := make([]int, n)
		for i := range out {
			out[i] = int(int32(binary.LittleEndian.Uint32(payload[i*4:])))
		}
		return out, nil
	case 'l':
		out := make([]int64, n)
		for i := range out {
			out[i] = int64(binary.LittleEndian.Uint64(payload[i*8:]))
		}
		return out, nil
	case 'b':
		out := make([]bool, n)
		for i := range out {
			out[i] = payload[i] != 0
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported array code %q", code)
	}
}

func walkFBX(n *fbxNode, f func(*fbxNode)) {
	f(n)
	for _, c := range n.children {
		walkFBX(c, f)
	}
}

func meshGeometryFromFBXNode(n *fbxNode) (kmdata.MeshGeometry, bool) {
	var g kmdata.MeshGeometry
	if len(n.props) > 0 {
		if id, ok := n.props[0].(int64); ok {
			g.FBXID = id
		}
	}
	if len(n.props) > 1 {
		if name, ok := n.props[1].(string); ok {
			g.Name = cleanFBXName(name)
		}
	}
	var rawVerts []float64
	var rawIdx []int
	var rawUV []float64
	var rawUVIndex []int
	var uvMapping, uvRef string
	for _, c := range n.children {
		switch c.name {
		case "Vertices":
			if len(c.props) > 0 {
				rawVerts, _ = c.props[0].([]float64)
			}
		case "PolygonVertexIndex":
			if len(c.props) > 0 {
				rawIdx, _ = c.props[0].([]int)
			}
		case "LayerElementUV":
			for _, uv := range c.children {
				switch {
				case uv.name == "MappingInformationType" && len(uv.props) > 0:
					uvMapping, _ = uv.props[0].(string)
				case uv.name == "ReferenceInformationType" && len(uv.props) > 0:
					uvRef, _ = uv.props[0].(string)
				case uv.name == "UV" && len(uv.props) > 0:
					rawUV, _ = uv.props[0].([]float64)
				case uv.name == "UVIndex" && len(uv.props) > 0:
					rawUVIndex, _ = uv.props[0].([]int)
				}
			}
		}
	}
	if len(rawVerts) < 3 || len(rawIdx) < 3 {
		return kmdata.MeshGeometry{}, false
	}
	g.Vertices = make([][3]float64, 0, len(rawVerts)/3)
	for i := 0; i+2 < len(rawVerts); i += 3 {
		g.Vertices = append(g.Vertices, [3]float64{rawVerts[i], rawVerts[i+1], rawVerts[i+2]})
	}
	for i := 0; i+1 < len(rawUV); i += 2 {
		g.UVs = append(g.UVs, [2]float64{rawUV[i], rawUV[i+1]})
	}
	g.Indices, g.UVIndices = triangulateFBXPolygonIndices(rawIdx, rawUVIndex, uvMapping, uvRef)
	if len(g.Indices) == 0 {
		return kmdata.MeshGeometry{}, false
	}
	return g, true
}

func cleanFBXName(s string) string {
	if i := strings.IndexByte(s, 0); i >= 0 {
		return s[:i]
	}
	return s
}

func triangulateFBXPolygonIndices(raw, rawUV []int, uvMapping, uvRef string) ([]int, []int) {
	var out, uvOut []int
	var poly, uvPoly []int
	polyVertex := 0
	flush := func() {
		if len(poly) >= 3 {
			for i := 1; i+1 < len(poly); i++ {
				out = append(out, poly[0], poly[i], poly[i+1])
				if len(uvPoly) == len(poly) {
					uvOut = append(uvOut, uvPoly[0], uvPoly[i], uvPoly[i+1])
				}
			}
		}
		poly = poly[:0]
		uvPoly = uvPoly[:0]
	}
	for pi, idx := range raw {
		uvIdx := fbxUVIndex(idx, pi, polyVertex, rawUV, uvMapping, uvRef)
		if idx < 0 {
			poly = append(poly, -idx-1)
			if uvIdx >= 0 {
				uvPoly = append(uvPoly, uvIdx)
			}
			polyVertex++
			flush()
			continue
		}
		poly = append(poly, idx)
		if uvIdx >= 0 {
			uvPoly = append(uvPoly, uvIdx)
		}
		polyVertex++
	}
	flush()
	if len(uvOut) != len(out) {
		uvOut = nil
	}
	return out, uvOut
}

func fbxUVIndex(vertexIdx, polygonIdx, polygonVertex int, rawUV []int, mapping, ref string) int {
	vertex := vertexIdx
	if vertex < 0 {
		vertex = -vertex - 1
	}
	switch mapping {
	case "ByPolygonVertex":
		if ref == "IndexToDirect" {
			if polygonIdx >= 0 && polygonIdx < len(rawUV) {
				return rawUV[polygonIdx]
			}
			return -1
		}
		return polygonVertex
	case "ByVertice":
		if ref == "IndexToDirect" {
			if vertex >= 0 && vertex < len(rawUV) {
				return rawUV[vertex]
			}
			return -1
		}
		return vertex
	default:
		return -1
	}
}
