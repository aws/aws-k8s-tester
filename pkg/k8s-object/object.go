// Package k8sobject implements Kubernetes object utilities.
package k8sobject

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
)

// ExtractTypeMeta extracts runtime.TypeMeta from encoded raw k8s object.
func ExtractTypeMeta(d []byte) (meta runtime.TypeMeta, err error) {
	data, tp := getEncType(d)
	switch tp {
	case unknownType:
		return runtime.TypeMeta{}, errors.New("unknown encoding")
	case protoType:
		var obj runtime.Unknown
		if err = obj.Unmarshal(data); err != nil {
			return runtime.TypeMeta{}, fmt.Errorf("failed to decode runtime.Unknown %v", err)
		}
		meta = obj.TypeMeta
	case jsonType:
		err = json.Unmarshal(data, &meta)
		if err != nil {
			return runtime.TypeMeta{}, fmt.Errorf("failed to decode runtime.TypeMeta %v", err)
		}
	default:
		return runtime.TypeMeta{}, fmt.Errorf("unknown type %v", tp)
	}
	return meta, err
}

var (
	protoPfx = []byte{0x6b, 0x38, 0x73, 0x00}
	jsonPfx  = "{["
)

func getEncType(d []byte) ([]byte, encType) {
	if len(d) < 4 {
		return d, unknownType
	}
	if bytes.Equal(d[:4], protoPfx) {
		return d[4:], protoType
	}
	idx := bytes.Index(d, protoPfx)
	if idx >= 0 && idx < len(d) {
		return d[idx:], protoType
	}

	var msg json.RawMessage
	idx = bytes.IndexAny(d, jsonPfx)
	for idx >= 0 && idx < len(d) {
		d = d[idx:]
		if len(d) < 2 {
			break
		}
		err := json.Unmarshal(d, &msg)
		if err == nil {
			d, err = msg.MarshalJSON()
			return d, jsonType
		}
		d = d[1:]
		idx = bytes.IndexAny(d, jsonPfx)
	}

	return d, unknownType
}

type encType uint8

const (
	unknownType encType = iota
	protoType
	jsonType
)

func (et encType) String() string {
	switch et {
	case unknownType:
		return "unknown"
	case protoType:
		return "protocol-buffer"
	case jsonType:
		return "json"
	default:
		panic("unknown")
	}
}
