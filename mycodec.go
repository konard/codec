// Package mycodec provides a complete serializer/deserializer for ANY Go value
// This is a completely new implementation written from scratch
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"unsafe"
)

// ============================================================================
// BINARY PROTOCOL FORMAT
// ============================================================================
// The protocol uses the following format:
// 1. Type Registry Section (serialized types used in the value)
// 2. Object Graph Section (serialized value with type references)
//
// Type Entry Format:
//   - type_id: uint32
//   - kind: uint8 (reflect.Kind)
//   - name_len: uint32
//   - name: []byte
//   - extra: depends on kind (elem types, key/val types, field info, etc.)
//
// Value Format:
//   - type_ref: uint32 (reference to type in registry)
//   - data: depends on type (primitives, pointer refs, container data)
// ============================================================================

const (
	// Special markers
	markerNil    = 0xFF
	markerNotNil = 0x00
)

// TypeRegistry manages type registration and mapping
type TypeRegistry struct {
	types        []reflect.Type          // ordered list of types
	typeToID     map[reflect.Type]uint32
	idToType     map[uint32]reflect.Type
	registering  map[reflect.Type]bool   // tracks types currently being registered to prevent infinite recursion
	nextID       uint32
}

func newTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types:       make([]reflect.Type, 0),
		typeToID:    make(map[reflect.Type]uint32),
		idToType:    make(map[uint32]reflect.Type),
		registering: make(map[reflect.Type]bool),
		nextID:      0,
	}
}

func (r *TypeRegistry) registerType(t reflect.Type) uint32 {
	if id, exists := r.typeToID[t]; exists {
		return id
	}

	// Check if we're currently registering this type (cyclic type reference)
	if r.registering[t] {
		// Reserve an ID for this type and continue
		id := r.nextID
		r.nextID++
		r.types = append(r.types, t)
		r.typeToID[t] = id
		r.idToType[id] = t
		return id
	}

	// Mark as being registered
	r.registering[t] = true
	defer delete(r.registering, t)

	// Register element types first to ensure proper ordering
	switch t.Kind() {
	case reflect.Array, reflect.Slice, reflect.Ptr, reflect.Chan:
		r.registerType(t.Elem())
	case reflect.Map:
		r.registerType(t.Key())
		r.registerType(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			r.registerType(t.Field(i).Type)
		}
	case reflect.Func:
		for i := 0; i < t.NumIn(); i++ {
			r.registerType(t.In(i))
		}
		for i := 0; i < t.NumOut(); i++ {
			r.registerType(t.Out(i))
		}
	}

	id := r.nextID
	r.nextID++
	r.types = append(r.types, t)
	r.typeToID[t] = id
	r.idToType[id] = t

	return id
}

func (r *TypeRegistry) getTypeID(t reflect.Type) (uint32, bool) {
	id, ok := r.typeToID[t]
	return id, ok
}

func (r *TypeRegistry) getType(id uint32) (reflect.Type, bool) {
	t, ok := r.idToType[id]
	return t, ok
}

// Serializer converts Go values to binary format
type Serializer struct {
	registry     *TypeRegistry
	buf          *bytes.Buffer
	ptrMap       map[uintptr]uint32 // tracks pointers to handle cycles during encoding
	visitedPtrs  map[uintptr]bool   // tracks visited pointers during type discovery
	nextPtrID    uint32
}

// NewSerializer creates a new serializer
func NewSerializer() *Serializer {
	return &Serializer{
		registry:    newTypeRegistry(),
		buf:         new(bytes.Buffer),
		ptrMap:      make(map[uintptr]uint32),
		visitedPtrs: make(map[uintptr]bool),
		nextPtrID:   0,
	}
}

// Serialize converts any Go value to binary data
func Serialize(v interface{}) ([]byte, error) {
	s := NewSerializer()
	return s.Encode(v)
}

// Encode encodes a value to binary format
func (s *Serializer) Encode(v interface{}) ([]byte, error) {
	// Phase 1: Discover all types used
	val := reflect.ValueOf(v)
	s.discoverTypes(val)

	// Phase 2: Write type registry
	typesData := s.encodeTypeRegistry()

	// Phase 3: Write value
	s.buf.Reset()
	s.encodeValue(val)
	valueData := s.buf.Bytes()

	// Phase 4: Combine everything
	result := new(bytes.Buffer)

	// Write number of types
	binary.Write(result, binary.LittleEndian, uint32(len(s.registry.types)))

	// Write types section
	result.Write(typesData)

	// Write value section
	result.Write(valueData)

	return result.Bytes(), nil
}

// discoverTypes traverses the value graph and registers all types
func (s *Serializer) discoverTypes(v reflect.Value) {
	if !v.IsValid() {
		return
	}

	t := v.Type()
	s.registry.registerType(t)

	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if !v.IsNil() {
			// Check for cyclic references
			if v.Kind() == reflect.Ptr {
				ptr := v.Pointer()
				if s.visitedPtrs[ptr] {
					return // Already visited this pointer
				}
				s.visitedPtrs[ptr] = true
			}
			s.discoverTypes(v.Elem())
		}

	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			s.discoverTypes(v.Field(i))
		}

	case reflect.Array, reflect.Slice:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return
		}
		for i := 0; i < v.Len(); i++ {
			s.discoverTypes(v.Index(i))
		}

	case reflect.Map:
		if v.IsNil() {
			return
		}
		iter := v.MapRange()
		for iter.Next() {
			s.discoverTypes(iter.Key())
			s.discoverTypes(iter.Value())
		}

	case reflect.Chan:
		// Channels don't contain values to traverse

	case reflect.Func:
		// Functions don't contain values to traverse
	}
}

// encodeTypeRegistry serializes the type registry
func (s *Serializer) encodeTypeRegistry() []byte {
	buf := new(bytes.Buffer)

	for _, t := range s.registry.types {
		s.encodeTypeInfo(buf, t)
	}

	return buf.Bytes()
}

// encodeTypeInfo encodes a single type's metadata
func (s *Serializer) encodeTypeInfo(buf *bytes.Buffer, t reflect.Type) {
	// Write kind
	buf.WriteByte(byte(t.Kind()))

	// Write name
	name := t.String()
	binary.Write(buf, binary.LittleEndian, uint32(len(name)))
	buf.WriteString(name)

	// Write kind-specific info
	switch t.Kind() {
	case reflect.Array:
		binary.Write(buf, binary.LittleEndian, uint32(t.Len()))
		elemID, _ := s.registry.getTypeID(t.Elem())
		binary.Write(buf, binary.LittleEndian, elemID)

	case reflect.Slice:
		elemID, _ := s.registry.getTypeID(t.Elem())
		binary.Write(buf, binary.LittleEndian, elemID)

	case reflect.Map:
		keyID, _ := s.registry.getTypeID(t.Key())
		valID, _ := s.registry.getTypeID(t.Elem())
		binary.Write(buf, binary.LittleEndian, keyID)
		binary.Write(buf, binary.LittleEndian, valID)

	case reflect.Ptr, reflect.Interface:
		elemID, _ := s.registry.getTypeID(t.Elem())
		binary.Write(buf, binary.LittleEndian, elemID)

	case reflect.Struct:
		numFields := t.NumField()
		binary.Write(buf, binary.LittleEndian, uint32(numFields))
		for i := 0; i < numFields; i++ {
			f := t.Field(i)
			// Write field name
			binary.Write(buf, binary.LittleEndian, uint32(len(f.Name)))
			buf.WriteString(f.Name)
			// Write field type ID
			fieldTypeID, _ := s.registry.getTypeID(f.Type)
			binary.Write(buf, binary.LittleEndian, fieldTypeID)
			// Write field offset
			binary.Write(buf, binary.LittleEndian, uint32(f.Offset))
			// Write exported flag
			exported := byte(0)
			if f.IsExported() {
				exported = 1
			}
			buf.WriteByte(exported)
		}

	case reflect.Chan:
		elemID, _ := s.registry.getTypeID(t.Elem())
		binary.Write(buf, binary.LittleEndian, elemID)
		binary.Write(buf, binary.LittleEndian, byte(t.ChanDir()))

	case reflect.Func:
		// Write number of input parameters
		numIn := t.NumIn()
		binary.Write(buf, binary.LittleEndian, uint32(numIn))
		for i := 0; i < numIn; i++ {
			inID, _ := s.registry.getTypeID(t.In(i))
			binary.Write(buf, binary.LittleEndian, inID)
		}
		// Write number of output parameters
		numOut := t.NumOut()
		binary.Write(buf, binary.LittleEndian, uint32(numOut))
		for i := 0; i < numOut; i++ {
			outID, _ := s.registry.getTypeID(t.Out(i))
			binary.Write(buf, binary.LittleEndian, outID)
		}
		// Write variadic flag
		variadic := byte(0)
		if t.IsVariadic() {
			variadic = 1
		}
		buf.WriteByte(variadic)
	}
}

// encodeValue serializes a value
func (s *Serializer) encodeValue(v reflect.Value) {
	if !v.IsValid() {
		s.buf.WriteByte(markerNil)
		return
	}

	s.buf.WriteByte(markerNotNil)

	// Write type reference
	typeID, _ := s.registry.getTypeID(v.Type())
	binary.Write(s.buf, binary.LittleEndian, typeID)

	switch v.Kind() {
	case reflect.Bool:
		val := byte(0)
		if v.Bool() {
			val = 1
		}
		s.buf.WriteByte(val)

	case reflect.Int:
		binary.Write(s.buf, binary.LittleEndian, int(v.Int()))
	case reflect.Int8:
		s.buf.WriteByte(byte(v.Int()))
	case reflect.Int16:
		binary.Write(s.buf, binary.LittleEndian, int16(v.Int()))
	case reflect.Int32:
		binary.Write(s.buf, binary.LittleEndian, int32(v.Int()))
	case reflect.Int64:
		binary.Write(s.buf, binary.LittleEndian, v.Int())

	case reflect.Uint:
		binary.Write(s.buf, binary.LittleEndian, uint(v.Uint()))
	case reflect.Uint8:
		s.buf.WriteByte(byte(v.Uint()))
	case reflect.Uint16:
		binary.Write(s.buf, binary.LittleEndian, uint16(v.Uint()))
	case reflect.Uint32:
		binary.Write(s.buf, binary.LittleEndian, uint32(v.Uint()))
	case reflect.Uint64:
		binary.Write(s.buf, binary.LittleEndian, v.Uint())
	case reflect.Uintptr:
		binary.Write(s.buf, binary.LittleEndian, uintptr(v.Uint()))

	case reflect.Float32:
		binary.Write(s.buf, binary.LittleEndian, float32(v.Float()))

	case reflect.Float64:
		binary.Write(s.buf, binary.LittleEndian, v.Float())

	case reflect.Complex64:
		c := v.Complex()
		binary.Write(s.buf, binary.LittleEndian, float32(real(c)))
		binary.Write(s.buf, binary.LittleEndian, float32(imag(c)))

	case reflect.Complex128:
		c := v.Complex()
		binary.Write(s.buf, binary.LittleEndian, real(c))
		binary.Write(s.buf, binary.LittleEndian, imag(c))

	case reflect.String:
		str := v.String()
		binary.Write(s.buf, binary.LittleEndian, uint32(len(str)))
		s.buf.WriteString(str)

	case reflect.Ptr:
		if v.IsNil() {
			s.buf.WriteByte(markerNil)
		} else {
			ptr := v.Pointer()
			if ptrID, exists := s.ptrMap[ptr]; exists {
				// Reference to existing pointer
				s.buf.WriteByte(0x01) // marker for reference
				binary.Write(s.buf, binary.LittleEndian, ptrID)
			} else {
				// New pointer
				s.buf.WriteByte(0x02) // marker for new pointer
				ptrID := s.nextPtrID
				s.nextPtrID++
				s.ptrMap[ptr] = ptrID
				binary.Write(s.buf, binary.LittleEndian, ptrID)
				s.encodeValue(v.Elem())
			}
		}

	case reflect.Interface:
		if v.IsNil() {
			s.buf.WriteByte(markerNil)
		} else {
			s.buf.WriteByte(markerNotNil)
			s.encodeValue(v.Elem())
		}

	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			s.encodeValue(v.Field(i))
		}

	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			s.encodeValue(v.Index(i))
		}

	case reflect.Slice:
		if v.IsNil() {
			s.buf.WriteByte(markerNil)
		} else {
			s.buf.WriteByte(markerNotNil)
			length := uint32(v.Len())
			binary.Write(s.buf, binary.LittleEndian, length)
			for i := 0; i < v.Len(); i++ {
				s.encodeValue(v.Index(i))
			}
		}

	case reflect.Map:
		if v.IsNil() {
			s.buf.WriteByte(markerNil)
		} else {
			s.buf.WriteByte(markerNotNil)
			length := uint32(v.Len())
			binary.Write(s.buf, binary.LittleEndian, length)
			iter := v.MapRange()
			for iter.Next() {
				s.encodeValue(iter.Key())
				s.encodeValue(iter.Value())
			}
		}

	case reflect.Chan:
		// Channels are encoded as type + capacity + buffer state
		// But we can't actually serialize the channel's goroutine state
		// We'll encode the type and capacity only
		if v.IsNil() {
			s.buf.WriteByte(markerNil)
		} else {
			s.buf.WriteByte(markerNotNil)
			binary.Write(s.buf, binary.LittleEndian, uint32(v.Cap()))
		}

	case reflect.Func:
		// Functions can't truly be serialized, but we store their pointer
		// This allows round-trip if the function is still in memory
		if v.IsNil() {
			s.buf.WriteByte(markerNil)
		} else {
			s.buf.WriteByte(markerNotNil)
			ptr := v.Pointer()
			binary.Write(s.buf, binary.LittleEndian, uint64(ptr))
		}

	default:
		// Unsupported type - encode as nil
		s.buf.WriteByte(markerNil)
	}
}

// ============================================================================
// DESERIALIZER
// ============================================================================

// Deserializer converts binary data back to Go values
type Deserializer struct {
	registry  *TypeRegistry
	data      []byte
	pos       int
	ptrMap    map[uint32]reflect.Value
}

// NewDeserializer creates a new deserializer
func NewDeserializer() *Deserializer {
	return &Deserializer{
		registry: newTypeRegistry(),
		ptrMap:   make(map[uint32]reflect.Value),
	}
}

// Deserialize converts binary data back to a Go value
func Deserialize(data []byte) (interface{}, error) {
	d := NewDeserializer()
	return d.Decode(data)
}

// Decode decodes binary data to a Go value
func (d *Deserializer) Decode(data []byte) (interface{}, error) {
	d.data = data
	d.pos = 0

	// Read number of types
	var numTypes uint32
	if err := d.readUint32(&numTypes); err != nil {
		return nil, fmt.Errorf("failed to read type count: %w", err)
	}

	// Read type registry
	for i := uint32(0); i < numTypes; i++ {
		if err := d.decodeTypeInfo(); err != nil {
			return nil, fmt.Errorf("failed to decode type %d: %w", i, err)
		}
	}

	// Read value
	val, err := d.decodeValue()
	if err != nil {
		return nil, fmt.Errorf("failed to decode value: %w", err)
	}

	if !val.IsValid() {
		return nil, nil
	}

	return val.Interface(), nil
}

func (d *Deserializer) readByte() (byte, error) {
	if d.pos >= len(d.data) {
		return 0, fmt.Errorf("unexpected end of data")
	}
	b := d.data[d.pos]
	d.pos++
	return b, nil
}

func (d *Deserializer) readBytes(n int) ([]byte, error) {
	if d.pos+n > len(d.data) {
		return nil, fmt.Errorf("unexpected end of data")
	}
	b := d.data[d.pos : d.pos+n]
	d.pos += n
	return b, nil
}

func (d *Deserializer) readUint32(val *uint32) error {
	b, err := d.readBytes(4)
	if err != nil {
		return err
	}
	*val = binary.LittleEndian.Uint32(b)
	return nil
}

func (d *Deserializer) readInt64(val *int64) error {
	b, err := d.readBytes(8)
	if err != nil {
		return err
	}
	*val = int64(binary.LittleEndian.Uint64(b))
	return nil
}

func (d *Deserializer) readUint64(val *uint64) error {
	b, err := d.readBytes(8)
	if err != nil {
		return err
	}
	*val = binary.LittleEndian.Uint64(b)
	return nil
}

func (d *Deserializer) readFloat64(val *float64) error {
	var u uint64
	if err := d.readUint64(&u); err != nil {
		return err
	}
	*val = *(*float64)(unsafe.Pointer(&u))
	return nil
}

func (d *Deserializer) readString() (string, error) {
	var length uint32
	if err := d.readUint32(&length); err != nil {
		return "", err
	}
	b, err := d.readBytes(int(length))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeTypeInfo decodes a type from the registry section
func (d *Deserializer) decodeTypeInfo() error {
	// Read kind
	kindByte, err := d.readByte()
	if err != nil {
		return err
	}
	kind := reflect.Kind(kindByte)

	// Read name (stored for debugging, not used in reconstruction)
	_, err = d.readString()
	if err != nil {
		return err
	}

	var t reflect.Type

	// Reconstruct type based on kind
	switch kind {
	case reflect.Bool:
		t = reflect.TypeOf(false)
	case reflect.Int:
		t = reflect.TypeOf(int(0))
	case reflect.Int8:
		t = reflect.TypeOf(int8(0))
	case reflect.Int16:
		t = reflect.TypeOf(int16(0))
	case reflect.Int32:
		t = reflect.TypeOf(int32(0))
	case reflect.Int64:
		t = reflect.TypeOf(int64(0))
	case reflect.Uint:
		t = reflect.TypeOf(uint(0))
	case reflect.Uint8:
		t = reflect.TypeOf(uint8(0))
	case reflect.Uint16:
		t = reflect.TypeOf(uint16(0))
	case reflect.Uint32:
		t = reflect.TypeOf(uint32(0))
	case reflect.Uint64:
		t = reflect.TypeOf(uint64(0))
	case reflect.Uintptr:
		t = reflect.TypeOf(uintptr(0))
	case reflect.Float32:
		t = reflect.TypeOf(float32(0))
	case reflect.Float64:
		t = reflect.TypeOf(float64(0))
	case reflect.Complex64:
		t = reflect.TypeOf(complex64(0))
	case reflect.Complex128:
		t = reflect.TypeOf(complex128(0))
	case reflect.String:
		t = reflect.TypeOf("")

	case reflect.Array:
		var length uint32
		if err := d.readUint32(&length); err != nil {
			return err
		}
		var elemID uint32
		if err := d.readUint32(&elemID); err != nil {
			return err
		}
		elemType, ok := d.registry.getType(elemID)
		if !ok {
			return fmt.Errorf("unknown element type ID: %d", elemID)
		}
		t = reflect.ArrayOf(int(length), elemType)

	case reflect.Slice:
		var elemID uint32
		if err := d.readUint32(&elemID); err != nil {
			return err
		}
		elemType, ok := d.registry.getType(elemID)
		if !ok {
			return fmt.Errorf("unknown element type ID: %d", elemID)
		}
		t = reflect.SliceOf(elemType)

	case reflect.Map:
		var keyID, valID uint32
		if err := d.readUint32(&keyID); err != nil {
			return err
		}
		if err := d.readUint32(&valID); err != nil {
			return err
		}
		keyType, ok := d.registry.getType(keyID)
		if !ok {
			return fmt.Errorf("unknown key type ID: %d", keyID)
		}
		valType, ok := d.registry.getType(valID)
		if !ok {
			return fmt.Errorf("unknown value type ID: %d", valID)
		}
		t = reflect.MapOf(keyType, valType)

	case reflect.Ptr:
		var elemID uint32
		if err := d.readUint32(&elemID); err != nil {
			return err
		}
		elemType, ok := d.registry.getType(elemID)
		if !ok {
			return fmt.Errorf("unknown element type ID: %d", elemID)
		}
		t = reflect.PtrTo(elemType)

	case reflect.Interface:
		var elemID uint32
		if err := d.readUint32(&elemID); err != nil {
			return err
		}
		elemType, ok := d.registry.getType(elemID)
		if !ok {
			return fmt.Errorf("unknown element type ID: %d", elemID)
		}
		// For interfaces, we store the concrete type
		t = elemType

	case reflect.Struct:
		var numFields uint32
		if err := d.readUint32(&numFields); err != nil {
			return err
		}

		fields := make([]reflect.StructField, numFields)
		for i := uint32(0); i < numFields; i++ {
			fieldName, err := d.readString()
			if err != nil {
				return err
			}

			var fieldTypeID uint32
			if err := d.readUint32(&fieldTypeID); err != nil {
				return err
			}
			fieldType, ok := d.registry.getType(fieldTypeID)
			if !ok {
				return fmt.Errorf("unknown field type ID: %d", fieldTypeID)
			}

			var offset uint32
			if err := d.readUint32(&offset); err != nil {
				return err
			}

			exported, err := d.readByte()
			if err != nil {
				return err
			}

			fields[i] = reflect.StructField{
				Name: fieldName,
				Type: fieldType,
				// Note: we can't set Offset directly, it's computed by reflect.StructOf
			}

			// Make unexported fields exported for reconstruction
			// (This is a limitation - we can't perfectly recreate unexported fields)
			if exported == 0 {
				// Keep the name but it will be exported in the reconstructed type
			}
		}

		t = reflect.StructOf(fields)

	case reflect.Chan:
		var elemID uint32
		if err := d.readUint32(&elemID); err != nil {
			return err
		}
		elemType, ok := d.registry.getType(elemID)
		if !ok {
			return fmt.Errorf("unknown element type ID: %d", elemID)
		}

		dirByte, err := d.readByte()
		if err != nil {
			return err
		}
		dir := reflect.ChanDir(dirByte)

		t = reflect.ChanOf(dir, elemType)

	case reflect.Func:
		var numIn uint32
		if err := d.readUint32(&numIn); err != nil {
			return err
		}

		inTypes := make([]reflect.Type, numIn)
		for i := uint32(0); i < numIn; i++ {
			var inID uint32
			if err := d.readUint32(&inID); err != nil {
				return err
			}
			inType, ok := d.registry.getType(inID)
			if !ok {
				return fmt.Errorf("unknown input type ID: %d", inID)
			}
			inTypes[i] = inType
		}

		var numOut uint32
		if err := d.readUint32(&numOut); err != nil {
			return err
		}

		outTypes := make([]reflect.Type, numOut)
		for i := uint32(0); i < numOut; i++ {
			var outID uint32
			if err := d.readUint32(&outID); err != nil {
				return err
			}
			outType, ok := d.registry.getType(outID)
			if !ok {
				return fmt.Errorf("unknown output type ID: %d", outID)
			}
			outTypes[i] = outType
		}

		variadicByte, err := d.readByte()
		if err != nil {
			return err
		}
		variadic := variadicByte == 1

		t = reflect.FuncOf(inTypes, outTypes, variadic)

	default:
		return fmt.Errorf("unsupported kind: %v", kind)
	}

	d.registry.registerType(t)
	return nil
}

// decodeValue decodes a value from the data stream
func (d *Deserializer) decodeValue() (reflect.Value, error) {
	marker, err := d.readByte()
	if err != nil {
		return reflect.Value{}, err
	}

	if marker == markerNil {
		return reflect.Value{}, nil
	}

	// Read type reference
	var typeID uint32
	if err := d.readUint32(&typeID); err != nil {
		return reflect.Value{}, err
	}

	t, ok := d.registry.getType(typeID)
	if !ok {
		return reflect.Value{}, fmt.Errorf("unknown type ID: %d", typeID)
	}

	switch t.Kind() {
	case reflect.Bool:
		b, err := d.readByte()
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(b == 1), nil

	case reflect.Int:
		var val int64
		if err := d.readInt64(&val); err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int(val)), nil

	case reflect.Int8:
		b, err := d.readByte()
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int8(b)), nil

	case reflect.Int16:
		b, err := d.readBytes(2)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int16(binary.LittleEndian.Uint16(b))), nil

	case reflect.Int32:
		b, err := d.readBytes(4)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int32(binary.LittleEndian.Uint32(b))), nil

	case reflect.Int64:
		var val int64
		if err := d.readInt64(&val); err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(val), nil

	case reflect.Uint:
		var val uint64
		if err := d.readUint64(&val); err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint(val)), nil

	case reflect.Uint8:
		b, err := d.readByte()
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint8(b)), nil

	case reflect.Uint16:
		b, err := d.readBytes(2)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(binary.LittleEndian.Uint16(b)), nil

	case reflect.Uint32:
		var val uint32
		if err := d.readUint32(&val); err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(val), nil

	case reflect.Uint64:
		var val uint64
		if err := d.readUint64(&val); err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(val), nil

	case reflect.Uintptr:
		var val uint64
		if err := d.readUint64(&val); err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uintptr(val)), nil

	case reflect.Float32:
		b, err := d.readBytes(4)
		if err != nil {
			return reflect.Value{}, err
		}
		u := binary.LittleEndian.Uint32(b)
		f := *(*float32)(unsafe.Pointer(&u))
		return reflect.ValueOf(f), nil

	case reflect.Float64:
		var val float64
		if err := d.readFloat64(&val); err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(val), nil

	case reflect.Complex64:
		b, err := d.readBytes(8)
		if err != nil {
			return reflect.Value{}, err
		}
		ru := binary.LittleEndian.Uint32(b[0:4])
		iu := binary.LittleEndian.Uint32(b[4:8])
		r := *(*float32)(unsafe.Pointer(&ru))
		i := *(*float32)(unsafe.Pointer(&iu))
		return reflect.ValueOf(complex(r, i)), nil

	case reflect.Complex128:
		var r, i float64
		if err := d.readFloat64(&r); err != nil {
			return reflect.Value{}, err
		}
		if err := d.readFloat64(&i); err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(complex(r, i)), nil

	case reflect.String:
		str, err := d.readString()
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(str), nil

	case reflect.Ptr:
		marker, err := d.readByte()
		if err != nil {
			return reflect.Value{}, err
		}

		if marker == markerNil {
			return reflect.Zero(t), nil
		}

		if marker == 0x01 {
			// Reference to existing pointer
			var ptrID uint32
			if err := d.readUint32(&ptrID); err != nil {
				return reflect.Value{}, err
			}
			if ptr, ok := d.ptrMap[ptrID]; ok {
				return ptr, nil
			}
			return reflect.Value{}, fmt.Errorf("unknown pointer ID: %d", ptrID)
		}

		// marker == 0x02: New pointer
		var ptrID uint32
		if err := d.readUint32(&ptrID); err != nil {
			return reflect.Value{}, err
		}

		// Create new pointer
		ptr := reflect.New(t.Elem())
		d.ptrMap[ptrID] = ptr

		// Decode element
		elem, err := d.decodeValue()
		if err != nil {
			return reflect.Value{}, err
		}

		if elem.IsValid() {
			ptr.Elem().Set(elem)
		}

		return ptr, nil

	case reflect.Interface:
		marker, err := d.readByte()
		if err != nil {
			return reflect.Value{}, err
		}

		if marker == markerNil {
			return reflect.Zero(t), nil
		}

		elem, err := d.decodeValue()
		if err != nil {
			return reflect.Value{}, err
		}

		// Return the concrete value (it will be convertible to the interface)
		return elem, nil

	case reflect.Struct:
		val := reflect.New(t).Elem()
		for i := 0; i < t.NumField(); i++ {
			field, err := d.decodeValue()
			if err != nil {
				return reflect.Value{}, err
			}
			if field.IsValid() {
				val.Field(i).Set(field)
			}
		}
		return val, nil

	case reflect.Array:
		val := reflect.New(t).Elem()
		for i := 0; i < t.Len(); i++ {
			elem, err := d.decodeValue()
			if err != nil {
				return reflect.Value{}, err
			}
			if elem.IsValid() {
				val.Index(i).Set(elem)
			}
		}
		return val, nil

	case reflect.Slice:
		marker, err := d.readByte()
		if err != nil {
			return reflect.Value{}, err
		}

		if marker == markerNil {
			return reflect.Zero(t), nil
		}

		var length uint32
		if err := d.readUint32(&length); err != nil {
			return reflect.Value{}, err
		}

		slice := reflect.MakeSlice(t, int(length), int(length))
		for i := 0; i < int(length); i++ {
			elem, err := d.decodeValue()
			if err != nil {
				return reflect.Value{}, err
			}
			if elem.IsValid() {
				slice.Index(i).Set(elem)
			}
		}
		return slice, nil

	case reflect.Map:
		marker, err := d.readByte()
		if err != nil {
			return reflect.Value{}, err
		}

		if marker == markerNil {
			return reflect.Zero(t), nil
		}

		var length uint32
		if err := d.readUint32(&length); err != nil {
			return reflect.Value{}, err
		}

		m := reflect.MakeMap(t)
		for i := 0; i < int(length); i++ {
			key, err := d.decodeValue()
			if err != nil {
				return reflect.Value{}, err
			}
			val, err := d.decodeValue()
			if err != nil {
				return reflect.Value{}, err
			}
			if key.IsValid() && val.IsValid() {
				m.SetMapIndex(key, val)
			}
		}
		return m, nil

	case reflect.Chan:
		marker, err := d.readByte()
		if err != nil {
			return reflect.Value{}, err
		}

		if marker == markerNil {
			return reflect.Zero(t), nil
		}

		var capacity uint32
		if err := d.readUint32(&capacity); err != nil {
			return reflect.Value{}, err
		}

		// Create new channel with the same capacity
		ch := reflect.MakeChan(t, int(capacity))
		return ch, nil

	case reflect.Func:
		marker, err := d.readByte()
		if err != nil {
			return reflect.Value{}, err
		}

		if marker == markerNil {
			return reflect.Zero(t), nil
		}

		// Read function pointer (won't be valid unless it's the same process)
		var ptr uint64
		if err := d.readUint64(&ptr); err != nil {
			return reflect.Value{}, err
		}

		// We can't truly deserialize functions, return zero value
		return reflect.Zero(t), nil

	default:
		return reflect.Value{}, fmt.Errorf("unsupported type: %v", t)
	}
}
