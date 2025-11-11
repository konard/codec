package codec

import (
	"fmt"
	"io"
	"math"
	"math/bits"
	"reflect"
	"unsafe"

	"github.com/URALINNOVATSIYA/reflex"
)

type forwardPtr struct {
	cntrId   int
	elemId   int
	elemType reflect.Type
}

type Unserializer struct {
	typeRegistry *TypeRegistry
	id           int
	pos          int
	size         int
	data         []byte
	values       map[int]reflect.Value
	forwardPtrs  map[int]forwardPtr
}

func NewUnserializer() *Unserializer {
	return &Unserializer{
		typeRegistry: GetDefaultTypeRegistry(),
	}
}

func (u *Unserializer) WithOptions(options []any) *Unserializer {
	for _, option := range options {
		if v, ok := option.(*TypeRegistry); ok {
			u.WithTypeRegistry(v)
			continue
		}
		panic(fmt.Errorf("invalid option type %T", option))
	}
	return u
}

func (u *Unserializer) WithTypeRegistry(registry *TypeRegistry) *Unserializer {
	u.typeRegistry = registry
	return u
}

func (u *Unserializer) Decode(data []byte) (value any, err error) {
	if data == nil {
		return nil, io.ErrUnexpectedEOF
	}
	//defer func() {
	//	if e := recover(); e != nil {
	//		err = fmt.Errorf("%s", e)
	//	}
	//}()
	u.id = 0
	u.pos = 1 // skip version for now
	u.data = data
	u.size = len(data)
	u.values = make(map[int]reflect.Value)
	u.forwardPtrs = make(map[int]forwardPtr)
	if v := u.decode(); v.IsValid() {
		return v.Interface(), nil
	}
	return value, err
}

func (u *Unserializer) decode() reflect.Value {
	v := u.decodeNode(-1)
	u.restoreForwarPointers()
	return v
}

func (u *Unserializer) decodeId() int {
	return int(u.decodeCount(4))
}

func (u *Unserializer) decodeLength() int {
	return int(u.decodeCount(4))
}

func (u *Unserializer) decodeType() reflect.Type {
	return u.typeRegistry.typeById(int(u.decodeCount(3)))
}

func (u *Unserializer) decodeNode(parentContainerId int) reflect.Value {
	if u.top() == meta_ref {
		return u.decodeReference(nil, parentContainerId)
	}
	t := u.decodeType()
	return u.decodeValue(t, reflex.Zero(t), parentContainerId)
}

func (u *Unserializer) decodeContainer(containerType reflect.Type, containerValue reflect.Value) {
	containerValue = reflex.PtrAt(containerType, containerValue).Elem()
	u.values[u.id] = containerValue
	u.id++
	v := u.decodeValue(containerType, reflex.Zero(containerType), u.id-1)
	if v.IsValid() {
		containerValue.Set(v)
	}
}

func (u *Unserializer) decodeValue(t reflect.Type, v reflect.Value, parentContainerId int) reflect.Value {
	kind := v.Kind()
	switch kind {
	case reflect.Invalid:
		u.decodeNil()
	case reflect.Bool:
		u.decodeBool(v)
	case reflect.Uint8:
		u.decodeUint8(v)
	case reflect.Int8:
		u.decodeInt8(v)
	case reflect.Uint16:
		u.decodeUint16(v)
	case reflect.Int16:
		u.decodeInt16(v)
	case reflect.Uint32:
		u.decodeUint32(v)
	case reflect.Int32:
		u.decodeInt32(v)
	case reflect.Uint64:
		u.decodeUint64(v)
	case reflect.Int64:
		u.decodeInt64(v)
	case reflect.Uint:
		u.decodeUint(v)
	case reflect.Int:
		u.decodeInt(v)
	case reflect.Float32:
		u.decodeFloat32(v)
	case reflect.Float64:
		u.decodeFloat64(v)
	case reflect.Complex64:
		u.decodeComplex64(v)
	case reflect.Complex128:
		u.decodeComplex128(v)
	case reflect.Uintptr:
		u.decodeUintptr(v)
	case reflect.UnsafePointer:
		u.decodeUnsafePointer(v)
	default:
		if u.top() == meta_ref {
			return u.decodeReference(t, parentContainerId)
		}
		u.values[u.id] = v
		u.id++
		switch kind {
		case reflect.String:
			u.decodeString(v)
		case reflect.Chan:
			u.decodeChan(v)
		case reflect.Func:
			u.decodeFunc(v)
		case reflect.Array:
			u.decodeArray(v)
		case reflect.Slice:
			u.decodeSlice(v)
		case reflect.Map:
			u.decodeMap(v)
		case reflect.Struct:
			u.decodeStruct(v)
		case reflect.Interface:
			u.decodeInterface(v, parentContainerId)
		case reflect.Pointer:
			u.decodePointer(t.Elem(), v, parentContainerId)
		}
		return v
	}
	u.id++
	return v
}

func (u *Unserializer) decodeNil() {
	u.readByte()
}

func (u *Unserializer) decodeBool(v reflect.Value) {
	v.SetBool(u.readByte() == meta_tru)
}

func (u *Unserializer) decodeString(v reflect.Value) {
	v.SetString(string(u.readBytes(u.decodeLength())))
}

func (u *Unserializer) decodeUint8(v reflect.Value) {
	v.SetUint(uint64(u.readByte()))
}

func (u *Unserializer) decodeInt8(v reflect.Value) {
	v.SetInt(u2i(uint64(u.readByte())))
}

func (u *Unserializer) decodeUint16(v reflect.Value) {
	v.SetUint(u.decodeCount(2))
}

func (u *Unserializer) decodeInt16(v reflect.Value) {
	v.SetInt(u2i(u.decodeCount(2)))
}

func (u *Unserializer) decodeUint32(v reflect.Value) {
	v.SetUint(u.decodeCount(3))
}

func (u *Unserializer) decodeInt32(v reflect.Value) {
	v.SetInt(u2i(u.decodeCount(3)))
}

func (u *Unserializer) decodeUint64(v reflect.Value) {
	v.SetUint(u.decodeCount(4))
}

func (u *Unserializer) decodeInt64(v reflect.Value) {
	v.SetInt(u2i(u.decodeCount(4)))
}

func (u *Unserializer) decodeUint(v reflect.Value) {
	u.decodeUint64(v)
}

func (u *Unserializer) decodeInt(v reflect.Value) {
	u.decodeInt64(v)
}

func (u *Unserializer) decodeFloat32(v reflect.Value) {
	v.SetFloat(float64(u.readFloat32()))
}

func (u *Unserializer) readFloat32() float32 {
	return math.Float32frombits(bits.ReverseBytes32(uint32(u.decodeCount(3))))
}

func (u *Unserializer) decodeFloat64(v reflect.Value) {
	v.SetFloat(u.readFloat64())
}

func (u *Unserializer) readFloat64() float64 {
	return math.Float64frombits(bits.ReverseBytes64(u.decodeCount(4)))
}

func (u *Unserializer) decodeComplex64(v reflect.Value) {
	r := u.readFloat32()
	i := u.readFloat32()
	v.SetComplex(complex128(complex(r, i)))
}

func (u *Unserializer) decodeComplex128(v reflect.Value) {
	r := u.readFloat64()
	i := u.readFloat64()
	v.SetComplex(complex(r, i))
}

func (u *Unserializer) decodeUintptr(v reflect.Value) {
	u.decodeUint64(v)
}

func (u *Unserializer) decodeUnsafePointer(v reflect.Value) {
	v.SetPointer(unsafe.Pointer(uintptr(u.decodeCount(4))))
}

func (u *Unserializer) decodeChan(v reflect.Value) {
	if u.readByte() == meta_nil {
		return
	}
	cap := u.decodeLength()
	t := v.Type()
	if t.ChanDir() == reflect.BothDir {
		v.Set(reflect.MakeChan(t, cap))
	} else {
		el := t.Elem()
		biDirChan := reflect.ChanOf(reflect.BothDir, el)
		unDirChan := reflect.ChanOf(t.ChanDir(), el)
		ch := reflect.MakeChan(biDirChan, cap)
		v.Set(ch.Convert(unDirChan).Convert(t))
	}
}

func (u *Unserializer) decodeFunc(v reflect.Value) {
	if u.readByte() == meta_nonil {
		v.Set(u.typeRegistry.funcByType(v.Type()))
	}
}

func (u *Unserializer) decodeArray(v reflect.Value) {
	_ = u.readByte() // skip container mark
	for i, count := 0, v.Len(); i < count; i++ {
		elem := v.Index(i)
		u.decodeContainer(elem.Type(), elem)
	}
}

func (u *Unserializer) decodeSlice(v reflect.Value) {
	if u.readByte() == meta_nil {
		return
	}
	length := u.decodeLength()
	v.Set(reflect.MakeSlice(v.Type(), length, length))
	_ = u.readByte() // skip container mark
	for i := 0; i < length; i++ {
		elem := v.Index(i)
		u.decodeContainer(elem.Type(), elem)
	}
}

func (u *Unserializer) decodeMap(v reflect.Value) {
	if u.readByte() == meta_nil {
		return
	}
	length := u.decodeLength()
	v.Set(reflect.MakeMapWithSize(v.Type(), length))
	t := v.Type()
	keyType := t.Key()
	valueType := t.Elem()
	// Map keys and values are encoded without type information
	// We need to use the map's type to decode them
	for i := 0; i < length; i++ {
		key := reflex.Zero(keyType)
		u.id++
		u.values[u.id-1] = key
		u.decodeValue(keyType, key, u.id-2)

		value := reflex.Zero(valueType)
		u.id++
		u.values[u.id-1] = value
		u.decodeValue(valueType, value, u.id-2)

		v.SetMapIndex(key, value)
	}
}

func (u *Unserializer) decodeStruct(v reflect.Value) {
	_ = u.readByte() // skip container mark
	for i, count := 0, v.NumField(); i < count; i++ {
		field := v.Field(i)
		u.decodeContainer(field.Type(), field)
	}
}

func (u *Unserializer) decodeInterface(v reflect.Value, parentContainerId int) {
	elem := u.decodeNode(parentContainerId)
	if elem.IsValid() {
		v.Set(elem)
	}
}

func (u *Unserializer) decodePointer(elemType reflect.Type, v reflect.Value, parentContainerId int) {
	if u.readByte() == meta_nil {
		return
	}
	elemValue := reflex.Zero(elemType)
	v.Set(reflex.PtrAt(elemType, elemValue))
	elemValue = u.decodeValue(elemType, elemValue, parentContainerId)
	if !elemValue.IsValid() {
		return
	}
	u.setPtrValue(v, elemType, elemValue)
}

func (u *Unserializer) decodeReference(elemType reflect.Type, parentContainerId int) reflect.Value {
	_ = u.readByte() // skip reference indicator
	id := u.decodeId()
	if v, exists := u.values[id]; exists {
		u.id++
		if ptr, exists := u.forwardPtrs[id]; exists {
			return u.registerForwardPtr(u.id-2, parentContainerId, ptr.elemId, ptr.elemType)
		}
		return v
	}
	if elemType == nil {
		panic(fmt.Errorf("reference on node #%d is incorrect", id))
	}
	return u.registerForwardPtr(u.id-1, parentContainerId, id, elemType)
}

func (u *Unserializer) registerForwardPtr(ptrId, parentContainerId, elemId int, elemType reflect.Type) reflect.Value {
	cntrId := ptrId
	if ptrId == parentContainerId+1 || ptrId == parentContainerId+2 {
		cntrId = parentContainerId
	}
	u.forwardPtrs[ptrId] = forwardPtr{
		cntrId:   cntrId,
		elemId:   elemId,
		elemType: elemType,
	}
	return reflect.Value{}
}

func (u *Unserializer) decodeCount(sizeBits int) uint64 {
	cnt, length := bs2u(u.data[u.pos:], sizeBits)
	if length <= 0 {
		panic(io.ErrUnexpectedEOF)
	}
	u.pos += length
	return cnt
}

func (u *Unserializer) restoreForwarPointers() {
	for _, forwardPtr := range u.forwardPtrs {
		ptr := u.values[forwardPtr.cntrId]
		elemValue, exists := u.values[forwardPtr.elemId]
		if !exists {
			panic(fmt.Errorf("value #%d is not found", forwardPtr.elemId))
		}
		u.setPtrValue(ptr, forwardPtr.elemType, elemValue)
	}
}

func (u *Unserializer) setPtrValue(ptr reflect.Value, elemType reflect.Type, elemValue reflect.Value) {
	if elemValue.Kind() == reflect.Pointer && elemType.Kind() == reflect.Interface {
		ptr.Elem().Set(elemValue)
	} else {
		ptr.Set(reflex.PtrAt(elemType, elemValue))
	}
}

func (u *Unserializer) top() byte {
	return u.data[u.pos]
}

func (u *Unserializer) readByte() byte {
	u.pos++
	if u.pos > u.size {
		panic(io.ErrUnexpectedEOF)
	}
	return u.data[u.pos-1]
}

func (u *Unserializer) readBytes(count int) []byte {
	u.pos += count
	if u.pos > u.size {
		panic(io.ErrUnexpectedEOF)
	}
	return u.data[u.pos-count : u.pos]
}

func Unserialize(data []byte, options ...any) (any, error) {
	return NewUnserializer().
		WithOptions(options).
		Decode(data)
}

/*import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"reflect"
	"unsafe"
)

type Unserializer struct {
	refs             map[uint32]reflect.Value
	rels             map[uint32]uint32
	cnt              uint32
	data             []byte
	pos              int
	size             int
	structCodingMode int
}

func NewUnserializer(structCodingMode int) *Unserializer {
	return &Unserializer{structCodingMode: structCodingMode}
}

func (u *Unserializer) Decode(data []byte) (value any, err error) {
	if data == nil {
		return nil, io.ErrUnexpectedEOF
	}
	defer func() {
		if e := recover(); e != nil {
			err = errors.New(e.(string))
		}
	}()
	u.refs = make(map[uint32]reflect.Value)
	u.rels = make(map[uint32]uint32)
	u.cnt = 0
	u.data = data
	u.pos = 1 // skip version for now
	u.size = len(data)
	var v reflect.Value
	v, err = u.decode(v)
	if err == nil && v.IsValid() {
		value = v.Interface()
	}
	return value, err
}

func (u *Unserializer) decode(value reflect.Value) (v reflect.Value, err error) {
	isNil := false
	t := u.data[u.pos]
	if t&mask == tType {
		if t&null != 0 {
			isNil = true
		}
		if t&custom == 0 {
			t = u.data[u.pos+int(t&0b11)+2]
		}
	}
	cnt := u.cnt
	proceed := true
	switch t & mask {
	case tType:
		v, err = u.decodeSerializable()
	case tNil:
		v = u.decodeNil()
	case tBool:
		v, err = u.decodeBool()
	case tInt8:
		v, err = u.decodeFixedInt()
	case tInt:
		v, err = u.decodeInt()
	case tFloat:
		v, err = u.decodeFloat()
	case tComplex:
		v, err = u.decodeComplex()
	case tString:
		v, err = u.decodeString()
	case tUintptr:
		v, err = u.decodeUintptr()
	case tChan:
		if t&tFunc == tFunc {
			v, err = u.decodeFunc()
		} else {
			v, err = u.decodeChan(isNil)
		}
	case tList:
		v, err = u.decodeList(isNil)
		proceed = false
	case tMap:
		v, err = u.decodeMap(isNil)
		proceed = false
	case tStruct:
		v, err = u.decodeStruct()
		proceed = false
	case tPointer:
		v, err = u.decodePointer(value, isNil || t&null != 0)
		proceed = false
	case tRef:
		v, err = u.decodeReference(value)
		proceed = false
	case tInterface:
		v, err = u.decodeInterface(isNil || t&null != 0)
		proceed = false
	default:
		return value, fmt.Errorf("unrecognized type %b", t)
	}
	if err != nil {
		return v, err
	}
	if value.IsValid() {
		setValue(value, v)
	} else {
		value = v
	}
	if proceed {
		u.refs[u.cnt] = value
		u.cnt++
	} else {
		u.refs[cnt] = value
	}
	return value, nil
}

func (u *Unserializer) decodeSerializable() (reflect.Value, error) {
	isPtr := u.data[u.pos-1] == tPointer
	v, l, _, err := u.decodeTypeWithLength(false)
	if err != nil {
		return v, err
	}
	length := int(l)
	u.pos += length
	if u.pos > u.size {
		return v, io.ErrUnexpectedEOF
	}
	t := v.Type()
	var obj reflect.Value
	if isPtr {
		obj = reflect.New(t)
	} else {
		obj = v
	}
	if !isSerializable(obj) {
		return v, fmt.Errorf("struct %q does not implement Serializable interface", t.Name())
	}
	s := obj.MethodByName("Unserialize").Call([]reflect.Value{reflect.ValueOf(u.data[u.pos-length : u.pos])})
	e := s[1].Interface()
	if e != nil {
		return v, e.(error)
	}
	obj = reflect.ValueOf(s[0].Interface())
	if obj.Kind() == reflect.Pointer {
		obj = obj.Elem()
	}
	return obj.Convert(t), nil
}

func (u *Unserializer) decodeNil() reflect.Value {
	u.pos++
	return reflect.Value{}
}

func (u *Unserializer) decodeBool() (reflect.Value, error) {
	v, err := u.decodeType()
	isTrue := (u.data[u.pos-1] & tru) != 0
	if err != nil {
		return v, err
	}
	if isTrue {
		changeValue(v, true)
	} else {
		changeValue(v, false)
	}
	return v, nil
}

func (u *Unserializer) decodeString() (reflect.Value, error) {
	v, length, _, err := u.decodeTypeWithLength(false)
	if err != nil {
		return v, err
	}
	if length > 0 {
		start := u.pos
		u.pos += int(length)
		if u.pos > u.size {
			return v, io.ErrUnexpectedEOF
		}
		str := u.data[start:u.pos]
		changeValue(v, *(*string)(unsafe.Pointer(&str)))
	}
	return v, nil
}

func (u *Unserializer) decodeFixedInt() (reflect.Value, error) {
	v, err := u.decodeType()
	if err != nil {
		return v, err
	}
	t := u.data[u.pos-1]
	bitSize := 8 << (t & 0b11)
	hasMeta := t&meta != 0
	var size int
	si := u.pos
	sb := u.data[si]
	if hasMeta {
		if bitSize == 64 {
			size = int((u.data[si] & 0b11100000) >> 5)
			u.data[si] &= 0b00011111
		} else if bitSize == 32 {
			size = int((u.data[si] & 0b11000000) >> 6)
			u.data[si] &= 0b00111111
		} else if bitSize == 16 {
			if hasMeta {
				size = 1
			} else {
				size = 2
			}
		} else {
			size = 1
		}
	} else {
		size = bitSize >> 3
	}
	i, err := u.readUint(size)
	u.data[si] = sb // fix the changed first byte
	if err != nil {
		return v, err
	}
	if t&signed == 0 {
		switch bitSize {
		case 64:
			changeValue(v, i)
		case 32:
			changeValue(v, uint32(i))
		case 16:
			changeValue(v, uint16(i))
		default:
			changeValue(v, uint8(i))
		}
	} else {
		neg := i&1 != 0
		i >>= 1
		switch bitSize {
		case 64:
			if neg {
				changeValue(v, ^int64(i))
			} else {
				changeValue(v, int64(i))
			}
		case 32:
			if neg {
				changeValue(v, ^int32(i))
			} else {
				changeValue(v, int32(i))
			}
		case 16:
			if neg {
				changeValue(v, ^int16(i))
			} else {
				changeValue(v, int16(i))
			}
		default:
			if neg {
				changeValue(v, ^int8(i))
			} else {
				changeValue(v, int8(i))
			}
		}
	}
	return v, nil
}

func (u *Unserializer) decodeInt() (reflect.Value, error) {
	v, i, t, err := u.decodeTypeWithLength(false)
	if err != nil {
		return v, err
	}
	if t&signed == 0 {
		changeValue(v, i)
	} else if i&1 != 0 {
		changeValue(v, ^int(i>>1))
	} else {
		changeValue(v, int(i>>1))
	}
	return v, nil
}

func (u *Unserializer) decodeFloat() (reflect.Value, error) {
	v, f, t, err := u.decodeTypeWithLength(false)
	if err != nil {
		return v, err
	}
	if t&wide != 0 {
		changeValue(v, math.Float64frombits(bits.ReverseBytes64(f)))
	} else {
		changeValue(v, math.Float32frombits(bits.ReverseBytes32(uint32(f))))
	}
	return v, nil
}

func (u *Unserializer) decodeComplex() (v reflect.Value, err error) {
	v, err = u.decodeType()
	if err != nil {
		return v, err
	}
	t := u.data[u.pos-1]
	var r, i reflect.Value
	if r, err = u.decodeFloat(); err != nil {
		return r, err
	}
	if i, err = u.decodeFloat(); err != nil {
		return i, err
	}
	if t&wide != 0 {
		changeValue(v, complex(r.Interface().(float64), i.Interface().(float64)))
	} else {
		changeValue(v, complex(r.Interface().(float32), i.Interface().(float32)))
	}
	return v, nil
}

func (u *Unserializer) decodeUintptr() (reflect.Value, error) {
	v, ptr, t, err := u.decodeTypeWithLength(false)
	if err != nil {
		return v, err
	}
	if t&raw != 0 {
		changeValue(v, unsafe.Pointer(uintptr(ptr)))
	} else {
		changeValue(v, uintptr(ptr))
	}
	return v, nil
}

func (u *Unserializer) decodeChan(isNil bool) (reflect.Value, error) {
	v, err := u.decodeType()
	if err != nil {
		return v, err
	}
	if isNil {
		return v, nil
	}
	size := int(u.data[u.pos]&0b111) + 1
	u.pos++
	buffSize, err := u.readUint(size)
	if err != nil {
		return v, err
	}
	t := v.Type()
	if t.ChanDir() == reflect.BothDir {
		return reflect.MakeChan(t, int(buffSize)), nil
	}
	el := t.Elem()
	biDirChan := reflect.ChanOf(reflect.BothDir, el)
	unDirChan := reflect.ChanOf(t.ChanDir(), el)
	v = reflect.MakeChan(biDirChan, int(buffSize))
	v = v.Convert(unDirChan).Convert(t)
	return v, nil
}

func (u *Unserializer) decodeFunc() (reflect.Value, error) {
	v, err := u.decodeType()
	return v, err
}

func (u *Unserializer) decodeList(isNil bool) (reflect.Value, error) {
	v, l, t, err := u.decodeTypeWithLength(isNil)
	if err != nil {
		return v, err
	}
	length := int(l)
	if t&fixed == 0 && !isNil {
		v = reflect.MakeSlice(v.Type(), length, length)
	}
	u.refs[u.cnt] = v
	u.cnt++
	for i := 0; i < length; i++ {
		if _, err = u.decode(v.Index(i)); err != nil {
			return reflect.Value{}, err
		}
	}
	return v, nil
}

func (u *Unserializer) decodeMap(isNil bool) (reflect.Value, error) {
	mp, l, _, err := u.decodeTypeWithLength(isNil)
	if err != nil {
		return mp, err
	}
	length := int(l)
	if !isNil {
		mp = reflect.MakeMapWithSize(mp.Type(), length)
	}
	u.refs[u.cnt] = mp
	u.cnt++
	var k, v reflect.Value
	for i := 0; i < length; i++ {
		if k, err = u.decode(reflect.Value{}); err != nil {
			return v, err
		}
		if v, err = u.decode(reflect.Value{}); err != nil {
			return v, err
		}
		mp.SetMapIndex(k, v)
	}
	return mp, nil
}

func (u *Unserializer) decodeStruct() (reflect.Value, error) {
	v, l, _, err := u.decodeTypeWithLength(false)
	if err != nil {
		return v, err
	}
	u.refs[u.cnt] = v
	u.cnt++
	length := int(l)
	switch u.structCodingMode {
	case StructCodingModeIndex:
		var field, fieldIndex reflect.Value
		for i := 0; i < length; i++ {
			fieldIndex, err = u.decodeInt()
			if err != nil {
				return v, err
			}
			field = v.Field(int(fieldIndex.Uint()))
			field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
			_, err = u.decode(field)
			if err != nil {
				return v, err
			}
		}
	case StructCodingModeName:
		var field, fieldIndex, fieldName reflect.Value
		for i := 0; i < length; i++ {
			fieldName, err = u.decodeString()
			if err != nil {
				return v, err
			}
			if fieldName.String() == "_" {
				fieldIndex, err = u.decodeInt()
				if err != nil {
					return v, err
				}
				field = v.Field(int(fieldIndex.Uint()))
			} else {
				field = v.FieldByName(fieldName.String())
			}
			if field.IsValid() {
				field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
			}
			_, err = u.decode(field)
			if err != nil {
				return v, err
			}
		}
	default:
		for i := 0; i < length; i++ {
			field := v.Field(i)
			field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
			_, err = u.decode(field)
			if err != nil {
				return v, err
			}
		}
	}
	return v, nil
}

func (u *Unserializer) decodePointer(value reflect.Value, isNil bool) (reflect.Value, error) {
	v, err := u.decodeType()
	if err != nil {
		return v, err
	}
	cnt := u.cnt
	u.refs[cnt] = v
	u.cnt++
	u.rels[u.cnt] = cnt
	var el reflect.Value
	if isNil {
		if !v.IsValid() {
			if el, err = u.decodeType(); err != nil {
				return v, err
			}
			v = reflect.Zero(u.pointerTo(el).Type())
		}
	} else if el, err = u.decode(reflect.Value{}); err != nil {
		return v, err
	} else if v.IsValid() {
		if el.IsValid() {
			changeValue(v, u.pointerTo(el).Interface())
		}
	} else if el.IsValid() {
		v = u.pointerTo(el)
	} else if value.IsValid() {
		if value.Kind() == reflect.Interface && !value.IsNil() {
			v = u.pointerTo(reflect.Zero(value.Type().Elem()))
		} else {
			v = reflect.Zero(value.Type())
		}
	} else {
		v = reflect.ValueOf((*any)(nil))
	}
	u.refs[cnt] = v
	return v, nil
}

func (u *Unserializer) decodeReference(value reflect.Value) (v reflect.Value, err error) {
	t := u.data[u.pos]
	if t&val != 0 { // forward references
		return u.decodePointedValue(value)
	}
	// back references
	var cnt uint32
	if v, cnt, err = u.extractReferenceValue(); err != nil {
		return v, err
	}
	v = u.pointerTo(v)
	u.refs[u.cnt] = v
	u.rels[cnt] = u.cnt
	u.cnt++
	return v, nil
}

func (u *Unserializer) decodePointedValue(value reflect.Value) (v reflect.Value, err error) {
	var cnt uint32
	if v, cnt, err = u.extractReferenceValue(); err != nil {
		return v, err
	}
	if value.IsValid() {
		value.Set(v)
		v = value
	}
	relCnt, exists := u.rels[cnt]
	if exists {
		r := u.refs[relCnt]
		r.Set(u.pointerTo(v))
	}
	u.refs[cnt] = v
	return v, nil
}

func (u *Unserializer) extractReferenceValue() (reflect.Value, uint32, error) {
	size := (u.data[u.pos] & 0b111) + 1
	u.pos++
	cnt, err := u.readUint(int(size))
	if err != nil {
		return reflect.Value{}, 0, err
	}
	c := uint32(cnt)
	v, ok := u.refs[c]
	if !ok {
		return v, 0, fmt.Errorf("invalid reference #%d", c)
	}
	return v, c, nil
}

func (u *Unserializer) pointerTo(v reflect.Value) reflect.Value {
	var p reflect.Value
	if v.IsValid() {
		if v.CanAddr() {
			p = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr()))
		} else {
			p = reflect.New(v.Type())
			p.Elem().Set(v)
		}
	} else {
		p = reflect.ValueOf((*any)(nil))
	}
	return p
}

func (u *Unserializer) decodeInterface(isNil bool) (reflect.Value, error) {
	v, err := u.decodeType()
	if err != nil {
		return v, err
	}
	u.refs[u.cnt] = v
	//u.cnt++
	if isNil {
		return v, nil
	}
	return u.decode(v)
}

func (u *Unserializer) decodeType() (reflect.Value, error) {
	typeSignature, err := u.readTypeSignature()
	if err != nil || len(typeSignature) == 1 && typeSignature[0] == tPointer {
		return reflect.Value{}, err
	}
	return tChecker.valueOf(typeSignature)
}

func (u *Unserializer) decodeTypeWithLength(isNil bool) (reflect.Value, uint64, byte, error) {
	v, err := u.decodeType()
	if err != nil {
		return v, 0, 0, err
	}
	t := u.data[u.pos-1]
	if isNil {
		return v, 0, t, nil
	}
	size := int((t & 0b111) + 1)
	length, err := u.readLength(size)
	if err != nil {
		return v, 0, 0, err
	}
	return v, length, t, nil
}

func (u *Unserializer) readLength(lengthSize int) (uint64, error) {
	length, err := u.readUint(lengthSize)
	if lengthSize > 4 && math.MaxUint == math.MaxUint32 {
		return 0, errors.New("int type has 32 bit size but serialized value has 64 bit size")
	}
	if err != nil {
		return 0, err
	}
	return length, nil
}

func (u *Unserializer) readUint(size int) (uint64, error) {
	var i uint64
	for ; size > 0; size-- {
		if u.pos >= u.size {
			return 0, io.ErrUnexpectedEOF
		}
		i = i<<8 | uint64(u.data[u.pos])
		u.pos++
	}
	return i, nil
}

func (u *Unserializer) readTypeSignature() ([]byte, error) {
	if u.pos >= u.size {
		return nil, io.ErrUnexpectedEOF
	}
	t := u.data[u.pos]
	u.pos++
	switch t & mask {
	case tType:
		size := int((t & 0b11) + 1)
		u.pos += size
		if u.pos > u.size {
			return nil, io.ErrUnexpectedEOF
		}
		b := append([]byte{}, u.data[u.pos-size-1:u.pos]...)
		ut, err := u.readTypeSignature()
		if err != nil {
			return nil, err
		}
		b[0] &= mask
		return append(b, ut...), nil
	case tList:
		return []byte{t & (mask | fixed)}, nil
	case tInt8, tInt16, tInt32, tInt64:
		return []byte{t & (mask | 0b11 | signed)}, nil
	case tInt:
		return []byte{t & (mask | signed)}, nil
	case tFloat, tComplex:
		return []byte{t & (mask | wide)}, nil
	case tUintptr:
		return []byte{t & (mask | raw)}, nil
	case tChan:
		if t&tFunc == tFunc {
			return []byte{tFunc}, nil
		}
		return []byte{t & (mask | 0b11)}, nil
	default:
		return []byte{t & mask}, nil
	}
}

func Unserialize(bytes []byte, options ...int) (any, error) {
	var structCodingMode int
	if len(options) > 0 {
		structCodingMode = options[0]
	} else {
		structCodingMode = GetStructCodingMode()
	}
	return NewUnserializer(structCodingMode).Decode(bytes)
}
*/
