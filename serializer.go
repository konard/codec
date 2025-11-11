package codec

import (
	"fmt"
	"math"
	"math/bits"
	"reflect"
	"unsafe"

	"github.com/URALINNOVATSIYA/reflex"
)

const (
	meta_ref   byte = 0b0000_0000 // pseudo type for referenced values
	meta_fls   byte = 0b0000_0001 // boolean false
	meta_tru   byte = 0b0000_0011 // boolean true
	meta_nil   byte = 0b0001_0000 // determines whether underlying value is nil
	meta_nonil byte = 0b0010_0000 // determines whether underlying value is not nil
	meta_cntr  byte = 0b0100_0000 // mark of structs or arrays
)

type Serializer struct {
	typeRegistry *TypeRegistry
	values       *graph
	nodeId       int
}

func NewSerializer() *Serializer {
	return &Serializer{
		typeRegistry: GetDefaultTypeRegistry(),
	}
}

func (s *Serializer) WithOptions(options []any) *Serializer {
	for _, option := range options {
		if v, ok := option.(*TypeRegistry); ok {
			s.WithTypeRegistry(v)
			continue
		}
		panic(fmt.Errorf("invalid option type %T", option))
	}
	return s
}

func (s *Serializer) WithTypeRegistry(registry *TypeRegistry) *Serializer {
	s.typeRegistry = registry
	return s
}

func (s *Serializer) Encode(v any) []byte {
	s.nodeId = 0
	s.values = newGraph()
	bytes := s.encode(reflect.ValueOf(v))
	return append([]byte{version}, bytes...)
}

func (s *Serializer) encode(v reflect.Value) []byte {
	s.traverse(-1, v)
	b := s.encodeNodes()
	return b
}

func (s *Serializer) nextNodeId() int {
	id := s.nodeId
	s.nodeId++
	return id
}

func (s *Serializer) address(v reflect.Value) valueAddr {
	if ptr := s.ptrOf(v); ptr != nil {
		return valueAddr{
			ptr,
			reflex.NameOf(v.Type()),
		}
	}
	return valueAddr{}
}

func (s *Serializer) ptrOf(v reflect.Value) unsafe.Pointer {
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Struct, reflect.Array:
		return reflex.PtrOf(v)
	case reflect.String, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Pointer:
		return reflex.DirPtrOf(v)
	default:
		return nil
	}
}

func (s *Serializer) registerContainer(v reflect.Value, nodeId, parentNodeId int) bool {
	addr := valueAddr{
		reflex.PtrOf(v),
		reflex.NameOf(v.Type()),
	}
	if containerId, exists := s.values.containerNodeAt(addr); exists {
		s.values.addNodeWithValue(nodeId, parentNodeId, nodeValue{v: v, cntr: addr})
		s.values.renumber(nodeId, containerId+1)
		s.values.visit(s.values.children(containerId)[0])
		return false
	}
	s.values.addNodeWithValue(nodeId, parentNodeId, nodeValue{cntr: addr})
	return true
}

func (s *Serializer) registerValue(v reflect.Value, parentNodeId int) int {
	addr := s.address(v)
	if !addr.isValid() {
		nodeId := s.nextNodeId()
		s.values.addNodeWithValue(nodeId, parentNodeId, nodeValue{v: v})
		return nodeId
	}
	if nodeId, exists := s.values.nodeAt(addr); exists {
		s.values.addNode(nodeId, parentNodeId)
		return -1
	}
	nodeId := s.nextNodeId()
	s.values.addNodeWithValue(nodeId, parentNodeId, nodeValue{
		v:    v,
		addr: addr,
	})
	return nodeId
}

func (s *Serializer) traverse(parentId int, v reflect.Value) {
	nodeId := s.registerValue(v, parentId)
	if nodeId < 0 {
		return
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		s.traverseList(v, nodeId)
	case reflect.Map:
		s.traverseMap(v, nodeId)
	case reflect.Struct:
		s.traverseStruct(v, nodeId)
	case reflect.Interface:
		s.traverseInterface(v, nodeId)
	case reflect.Pointer:
		s.traversePointer(v, nodeId)
	}
}

func (s *Serializer) traverseList(v reflect.Value, id int) {
	length := v.Len()
	for i := 0; i < length; i++ {
		elem := v.Index(i)
		elemId := s.nextNodeId()
		if s.registerContainer(elem, elemId, id) {
			s.traverse(elemId, elem)
		}
	}
}

func (s *Serializer) traverseMap(v reflect.Value, id int) {
	iter := v.MapRange()
	for iter.Next() {
		s.traverse(id, iter.Key())
		s.traverse(id, iter.Value())
	}
}

func (s *Serializer) traverseStruct(v reflect.Value, nodeId int) {
	fieldCount := v.NumField()
	for i := 0; i < fieldCount; i++ {
		field := v.Field(i)
		fieldId := s.nextNodeId()
		if s.registerContainer(field, fieldId, nodeId) {
			s.traverse(fieldId, field)
		}
		fieldId++
	}
}

func (s *Serializer) traverseInterface(v reflect.Value, nodeId int) {
	s.traverse(nodeId, v.Elem())
}

func (s *Serializer) traversePointer(v reflect.Value, nodeId int) {
	if v.IsNil() {
		return
	}
	elem := v.Elem()
	addr := valueAddr{
		reflex.DirPtrOf(v),
		reflex.NameOf(elem.Type()),
	}
	if containerId, exists := s.values.containerNodeAt(addr); exists {
		s.values.addNodeValue(nodeId, nodeValue{v: v})
		s.values.addNode(containerId, nodeId)
		return
	}
	s.values.updateNodeValue(nodeId, s.values.nodeValue(nodeId), nodeValue{cntr: addr})
	s.traverse(nodeId, elem)
}

func (s *Serializer) encodeNodes() []byte {
	return s.encodeNode(s.values.children(-1)[0])
}

func (s *Serializer) encodeNode(nodeId int) []byte {
	v := s.values.get(nodeId)
	if s.values.isVisited(nodeId) {
		return s.encodeReference(nodeId)
	}
	s.values.visit(nodeId)
	return append(s.encodeType(v), s.encodeValue(v, nodeId)...)
}

func (s *Serializer) encodeContainer(containerId int) []byte {
	nodeId := s.values.children(containerId)[0]
	v := s.values.get(nodeId)
	return s.visitValue(v, nodeId)
}

func (s *Serializer) encodeType(v reflect.Value) []byte {
	return u2bs(uint64(s.typeRegistry.typeIdByValue(v)), 3)
}

func (s *Serializer) visitValue(v reflect.Value, nodeId int) []byte {
	if s.values.isVisited(nodeId) {
		return s.encodeReference(nodeId)
	}
	s.values.visit(nodeId)
	return s.encodeValue(v, nodeId)
}

func (s *Serializer) encodeValue(v reflect.Value, nodeId int) []byte {
	switch v.Kind() {
	case reflect.Invalid:
		return s.encodeNil()
	case reflect.Bool:
		return s.encodeBool(v)
	case reflect.String:
		return s.encodeString(v)
	case reflect.Uint8:
		return s.encodeUint8(v)
	case reflect.Int8:
		return s.encodeInt8(v)
	case reflect.Uint16:
		return s.encodeUint16(v)
	case reflect.Int16:
		return s.encodeInt16(v)
	case reflect.Uint32:
		return s.encodeUint32(v)
	case reflect.Int32:
		return s.encodeInt32(v)
	case reflect.Uint64:
		return s.encodeUint64(v)
	case reflect.Int64:
		return s.encodeInt(v)
	case reflect.Uint:
		return s.encodeUint(v)
	case reflect.Int:
		return s.encodeInt(v)
	case reflect.Float32:
		return s.encodeFloat32(v)
	case reflect.Float64:
		return s.encodeFloat64(v)
	case reflect.Complex64:
		return s.encodeComplex64(v)
	case reflect.Complex128:
		return s.encodeComplex128(v)
	case reflect.Uintptr:
		return s.encodeUintptr(v)
	case reflect.UnsafePointer:
		return s.encodeUnsafePointer(v)
	case reflect.Chan:
		return s.encodeChan(v)
	case reflect.Func:
		return s.encodeFunc(v)
	case reflect.Array:
		return s.encodeArray(nodeId)
	case reflect.Slice:
		return s.encodeSlice(v, nodeId)
	case reflect.Map:
		return s.encodeMap(v, nodeId)
	case reflect.Struct:
		return s.encodeStruct(nodeId)
	case reflect.Interface:
		return s.encodeInterface(nodeId)
	case reflect.Pointer:
		return s.encodePointer(nodeId)
	}
	panic("unrecognized value kind")
}

func (s *Serializer) encodeNil() []byte {
	return []byte{meta_nil}
}

func (s *Serializer) encodeBool(v reflect.Value) []byte {
	if v.Bool() {
		return []byte{meta_tru}
	}
	return []byte{meta_fls}
}

func (s *Serializer) encodeString(v reflect.Value) []byte {
	return append(c2b(v.Len()), v.String()...)
}

func (s *Serializer) encodeUint8(v reflect.Value) []byte {
	return []byte{uint8(v.Uint())}
}

func (s *Serializer) encodeInt8(v reflect.Value) []byte {
	return []byte{uint8(i2u(v.Int()))}
}

func (s *Serializer) encodeUint16(v reflect.Value) []byte {
	return u2bs(v.Uint(), 2)
}

func (s *Serializer) encodeInt16(v reflect.Value) []byte {
	return u2bs(i2u(v.Int()), 2)
}

func (s *Serializer) encodeUint32(v reflect.Value) []byte {
	return u2bs(v.Uint(), 3)
}

func (s *Serializer) encodeInt32(v reflect.Value) []byte {
	return u2bs(i2u(v.Int()), 3)
}

func (s *Serializer) encodeUint64(v reflect.Value) []byte {
	return u2bs(v.Uint(), 4)
}

func (s *Serializer) encodeInt64(v reflect.Value) []byte {
	return u2bs(i2u(v.Int()), 4)
}

func (s *Serializer) encodeUint(v reflect.Value) []byte {
	return s.encodeUint64(v)
}

func (s *Serializer) encodeInt(v reflect.Value) []byte {
	return s.encodeInt64(v)
}

func (s *Serializer) encodeFloat32(v reflect.Value) []byte {
	return u2bs(uint64(bits.ReverseBytes32(math.Float32bits(float32(v.Float())))), 3)
}

func (s *Serializer) encodeFloat64(v reflect.Value) []byte {
	return u2bs(bits.ReverseBytes64(math.Float64bits(v.Float())), 4)
}

func (s *Serializer) encodeComplex64(v reflect.Value) []byte {
	c := v.Complex()
	r := s.encodeFloat32(reflect.ValueOf(float32(real(c))))
	i := s.encodeFloat32(reflect.ValueOf(float32(imag(c))))
	return append(r, i...)
}

func (s *Serializer) encodeComplex128(v reflect.Value) []byte {
	c := v.Complex()
	r := s.encodeFloat64(reflect.ValueOf(real(c)))
	i := s.encodeFloat64(reflect.ValueOf(imag(c)))
	return append(r, i...)
}

func (s *Serializer) encodeUintptr(v reflect.Value) []byte {
	return s.encodeUint64(v)
}

func (s *Serializer) encodeUnsafePointer(v reflect.Value) []byte {
	return u2bs(uint64(v.Pointer()), 4)
}

func (s *Serializer) encodeChan(v reflect.Value) []byte {
	if v.IsNil() {
		return []byte{meta_nil}
	}
	return append([]byte{meta_nonil}, c2b(v.Cap())...)
}

func (s *Serializer) encodeFunc(v reflect.Value) []byte {
	if v.IsNil() {
		return []byte{meta_nil}
	}
	return []byte{meta_nonil}
}

func (s *Serializer) encodeArray(nodeId int) []byte {
	b := []byte{meta_cntr}
	for _, cntrId := range s.values.children(nodeId) {
		s.values.visit(cntrId)
		b = append(b, s.encodeContainer(cntrId)...)
	}
	return b
}

func (s *Serializer) encodeSlice(v reflect.Value, nodeId int) []byte {
	if v.IsNil() {
		return []byte{meta_nil}
	}
	b := append([]byte{meta_nonil}, c2b(v.Len())...)
	b = append(b, meta_cntr)
	for _, cntrId := range s.values.children(nodeId) {
		s.values.visit(cntrId)
		b = append(b, s.encodeContainer(cntrId)...)
	}
	return b
}

func (s *Serializer) encodeMap(v reflect.Value, nodeId int) []byte {
	if v.IsNil() {
		return []byte{meta_nil}
	}
	b := append([]byte{meta_nonil}, c2b(v.Len())...)
	for _, id := range s.values.children(nodeId) {
		b = append(b, s.encodeValue(s.values.get(id), id)...)
	}
	return b
}

func (s *Serializer) encodeStruct(nodeId int) []byte {
	b := []byte{meta_cntr}
	for _, fieldId := range s.values.children(nodeId) {
		s.values.visit(fieldId)
		b = append(b, s.encodeContainer(fieldId)...)
	}
	return b
}

func (s *Serializer) encodeInterface(nodeId int) []byte {
	return s.encodeNode(s.values.children(nodeId)[0])
}

func (s *Serializer) encodePointer(nodeId int) []byte {
	childs := s.values.children(nodeId)
	if len(childs) == 0 {
		return []byte{meta_nil}
	}
	childId := childs[0]
	b := s.visitValue(s.values.get(childId), childId)
	return append([]byte{meta_nonil}, b...)
}

func (s *Serializer) encodeReference(id int) []byte {
	return append([]byte{meta_ref}, c2b(id)...)
}

func (s *Serializer) encodeSerializable(v reflect.Value) []byte {
	/*if isNil(v) {
		return s.encodeNil()
	}
	var b []byte
	if v.Kind() == reflect.Pointer {
		cnt, ok := s.refs[s.valueSignature(v.Elem())]
		if ok {
			return append([]byte{1}, s.encodeReference(cnt)...)
		}
		b = []byte{0}
	}
	body := v.MethodByName("Serialize").Call(nil)[0].Interface().([]byte)
	return append(b, body...)*/
	return nil
}

func Serialize(value any, options ...any) []byte {
	return NewSerializer().
		WithOptions(options).
		Encode(value)
}
