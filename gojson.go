package gojson

/*
    GoJson是对json的封装，用interface{} 屏蔽了对结构体的使用依赖。

   	Permission is hereby granted, free of charge, to any person obtaining a
	copy of this software and associated documentation files (the "Software"),
	to deal in the Software without restriction, including without limitation the
	rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
	sell copies of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:The above copyright
	notice and this permission notice shall be included in all copies or substantial
	portions of the Software.THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY
	OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
	OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
	IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
	DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
	OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
	USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

import (
	"bytes"
	sysjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	jsoniterator "github.com/json-iterator/go"
)

var json = jsoniterator.ConfigCompatibleWithStandardLibrary

var Debug = true

func debugf(format string, v ...interface{}) {
	if Debug {
		log.Println(fmt.Sprintf(format, v...))
	}
}

type GoJson struct {
	prev      *GoJson
	prevKey   string
	prevIndex int
	data      interface{}
	sync.RWMutex
}

// NewJson 从string, []byte, interface{}等对象创建GoJson对象。从结构体创建应使用：NewJsonFromStruct
func NewJson(data interface{}) *GoJson {
	switch v := data.(type) {
	case string:
		return NewJsonFromString(v)
	case []byte:
		return NewJsonFromBytes(v)
	default:
		return NewJsonFromData(v)
	}
}
func decodeJson(r io.Reader) (*GoJson, error) {
	var f interface{}
	decoder := json.NewDecoder(r)
	decoder.UseNumber() // UseNumber causes the Decoder to unmarshal a number into an interface{} as a Number instead of as a float64.
	if err := decoder.Decode(&f); err != nil {
		return nil, err
	}

	return &GoJson{data: f}, nil
}

// NewJsonFromBytes 从bytes对象创建GoJson对象。bytes对象必须是标准的json格式。
func NewJsonFromBytes(b []byte) *GoJson {
	js, err := decodeJson(bytes.NewReader(b))
	if err != nil {
		// errstr := fmt.Sprintf("js解析失败：%s", b)
		return &GoJson{}
	}
	return js
}

// NewJsonFromString 从一个字符串对象创建GoJson对象
func NewJsonFromString(str string) *GoJson {
	js, err := decodeJson(strings.NewReader(str))
	if err != nil {
		// errstr := fmt.Sprintf("js解析失败：%s", str)
		return &GoJson{}
	}
	return js
}

func NewErrJson(errcode int, errmsg string) *GoJson {
	result := NewJsonFromString("{}")
	result.Set("err_msg", errmsg)
	result.Set("err", errcode)

	return result
}

// NewJsonFromStruct 从一个结构体对象创建GoJson对象
func NewJsonFromStruct(b interface{}) *GoJson {
	var f interface{}
	bytesArr, err := json.Marshal(b)
	if err != nil {
		// errstr := fmt.Sprintf("js解析失败：%v", err)
		return &GoJson{}
	}

	err = json.Unmarshal(bytesArr, &f)
	if err != nil {
		// errstr := fmt.Sprintf("js解析失败：%v", err)
		return &GoJson{}
	}

	return &GoJson{data: f}
}

// NewJsonFromData 从interface{}创建一个json。并不会做什么处理，只是用来包装原始数据。
func NewJsonFromData(d interface{}) *GoJson {
	return &GoJson{data: d}
}

func getMap(key string, mapBody interface{}) (interface{}, bool) {
	switch v := mapBody.(type) {
	case map[string]interface{}:
		return v[key], true
	case Dict:
		return v[key], true
	default:
		return nil, false
	}
}

// 取出json object中的所有key
func (j *GoJson) Keys() []string {
	var result []string

	if j.data == nil {
		return result
	}

	jsonMap, ok := j.data.(map[string]interface{})
	if !ok {
		panic(fmt.Sprintf("Input invalid error, your input json is: %s", ToString(j)))
	}
	for key, _ := range jsonMap {
		result = append(result, key)
	}
	return result
}

func setMap(key string, mapBody, data interface{}) bool {
	var val interface{}
	if value, ok := data.(*GoJson); ok {
		val = value.Value()
	} else {
		val = data
	}

	switch v := mapBody.(type) {
	case map[string]interface{}:
		v[key] = val
		return true
	case Dict:
		v[key] = val
		return true
	default:
		return false
	}
}

func getSlice(key int, sliceBody interface{}) (interface{}, bool) {
	switch v := sliceBody.(type) {
	case []interface{}:
		return v[key], true
	case List:
		return v[key], true
	default:
		return nil, false
	}
}

func appendSlice(sliceBody, data interface{}) (interface{}, bool) {
	var val interface{}
	if value, ok := data.(*GoJson); ok {
		val = value.Value()
	} else {
		val = data
	}

	switch v := sliceBody.(type) {
	case []interface{}:
		v = append(v, val)
		return v, true
	case List:
		v = append(v, val)
		return v, true
	default:
		return nil, false
	}
}

func setSlice(index int, sliceBody, data interface{}) bool {
	var val interface{}
	if value, ok := data.(*GoJson); ok {
		val = value.Value()
	} else {
		val = data
	}

	switch v := sliceBody.(type) {
	case []interface{}:
		v[index] = val
		return true
	case List:
		v[index] = val
		return true
	default:
		return false
	}
}

func insertSlice(index int, sliceBody, data interface{}) (interface{}, bool) {
	var val interface{}
	if value, ok := data.(*GoJson); ok {
		val = value.Value()
	} else {
		val = data
	}

	switch v := sliceBody.(type) {
	case []interface{}:
		rear := append([]interface{}{}, v[index:]...)
		v = append(v[0:index], val)
		return append(v, rear...), true
	case List:
		rear := append([]interface{}{}, v[index:]...)
		v = append(v[0:index], val)
		return append(v, rear...), true
	default:
		return nil, false
	}
}

// Get 获取一个key值。返回GoJson对象
func (j *GoJson) Get(key string) *GoJson {
	m, ok := getMap(key, j.data)
	if !ok {
		return &GoJson{
			prev:    j,
			prevKey: key,
			data:    nil,
		}
	}

	return &GoJson{
		prev:    j,
		prevKey: key,
		data:    m,
	}
}

// 获得key对应的string，若key不存在，则返回空字符串
func (j *GoJson) GetString(key string) string {
	m, ok := getMap(key, j.data)
	if !ok {
		return ""
	}

	g := &GoJson{
		prev:    j,
		prevKey: key,
		data:    m,
	}
	return ToString(g)
}

// maintainParent 维护这个节点与父节点的关系
func maintainParent(child *GoJson) {
	if child.prev == nil {
		return
	}

	switch child.prev.Value().(type) {
	case map[string]interface{}:
		child.prev.Set(child.prevKey, child)
	case []interface{}:
		child.prev.Set(child.prevIndex, child)
	}
}

// Append 往数组中添加值并返回自身，当json不为slice，将直接返回自身
func (j *GoJson) Append(val interface{}) *GoJson {
	var v interface{}
	if value, ok := val.(*GoJson); ok {
		v = value.Value()
	} else {
		v = val
	}

	data, ok := appendSlice(j.data, v)
	if !ok {
		log.Println(fmt.Sprintf("%v is not slice cannot append", j.data))
		return j
	}
	j.data = data

	maintainParent(j)
	return j
}

// Insert 往数组中添加值，当json不为slice，返回自身，什么都不会发生
func (j *GoJson) Insert(index int, val interface{}) *GoJson {
	v, ok := insertSlice(index, j.data, val)
	if !ok {
		log.Println(fmt.Sprintf("%v is not slice cannot insert", j.data))
		return j
	}
	j.data = v
	maintainParent(j)
	return j
}

// IsNil 判定data是不是空，常用来检测NewJson, Get, Index的结果是否为空
func (j *GoJson) IsNil() bool {
	if j.data == nil {
		return true
	}
	return false
}

// IsSlice 判定GoJson对象源数据是不是数组结构
func (j *GoJson) IsSlice() bool {
	switch j.data.(type) {
	case List:
		return true
	case []interface{}:
		return true
	default:
		return false
	}
}

// IsMap 判定GoJson对象源数据是不是k-v结构
func (j *GoJson) IsMap() bool {
	switch j.data.(type) {
	case Dict, map[string]interface{}:
		return true
	default:
		return false
	}
}

// Index 传入位置，获取slice对应位置的GoJson对象。如果这个对象不存在，返回的GoJson对象 IsNil将为true
func (j *GoJson) Index(key int) *GoJson {
	v, ok := getSlice(key, j.data)
	if !ok {
		return &GoJson{
			prev:      j,
			prevIndex: key,
			data:      nil,
		}
	}

	return &GoJson{
		prev:      j,
		prevIndex: key,
		data:      v,
	}
}

// Set 对当前的GoJson对象对应key设置值
func (j *GoJson) Set(key interface{}, val interface{}) *GoJson {
	switch v := key.(type) {
	case string:
		ok := setMap(v, j.data, val)
		if !ok {
			log.Println(fmt.Sprintf("%v is not map cannot set", j.data))
			return j
		}
	case int:
		ok := setSlice(v, j.data, val)
		if !ok {
			log.Println(fmt.Sprintf("%v is not map cannot set", j.data))
			return j
		}
	}
	return j
}

// Remove 删除GoJson的一个key。目前不能操作slice，只能操作k-v结构
func (j *GoJson) Remove(key interface{}) *GoJson {
	switch keyVal := key.(type) {
	case string:
		if j.IsMap() {
			switch v := j.data.(type) {
			case map[string]interface{}:
				delete(v, keyVal)
			case Dict:
				delete(v, keyVal)
			}
		}
	case int:
		// 暂时未实现rm slice功能
	}
	return j
}

// Value 返回GoJson对象的真实数据
func (j *GoJson) Value() interface{} {
	v := j.data
	return v
}

// Len 返回数组对象的长度，如果源数据不是数组，则返回0
func (j *GoJson) Len() int {
	switch v := j.data.(type) {
	case []interface{}:
		return len(v)
	case List:
		return len(v)
	default:
		return 0
	}
}

// String方法返回GoJson对象的字符串值
func (j *GoJson) String() string {
	if j.data == nil {
		return ""
	}
	switch j.data.(type) {
	case map[string]interface{}, []interface{}, Dict, List:
		buffer := &bytes.Buffer{}
		encoder := json.NewEncoder(buffer)
		//encoder.SetEscapeHTML(false)
		err := encoder.Encode(j.data)
		if err != nil {
			log.Println("convert to String is error", err)
			return ""
		}
		return buffer.String()
	default:
		return ToString(j.data)
	}
}

// Bytes 返回GoJson对象的bytes值
func (j *GoJson) Bytes() []byte {
	if j.data == nil {
		return []byte("")
	}
	switch j.data.(type) {
	case map[string]interface{}, []interface{}, Dict, List:
		result, err := json.Marshal(j.data)
		if err != nil {
			log.Println("convert to bytes is error", err)
			return []byte("")
		}
		return result
	default:
		return []byte(ToString(j.data))
	}
}

// Int 返回GoJson对象的源数据, 并尝试转换为int
func (j *GoJson) Int() (int, error) {
	v := j.data
	if v == nil {
		return 0, errors.New(fmt.Sprintf("%v is not int", j.data))
	}
	return ToInt(v)
}

// Float64 返回GoJson对象的源数据, 并尝试转换为float64
func (j *GoJson) Float64() (float64, error) {
	v := j.data
	if v == nil {
		return 0, errors.New(fmt.Sprintf("%v is not float64", j.data))
	}
	return ToFloat64(v)
}

// Bool 返回GoJson对象的源数据, 并尝试转换为bool
func (j *GoJson) Bool() (bool, error) {
	return ToBool(j.data)
}

// Array 返回数组对象的源数据，如果源数据不是数组，则返回error
func (j *GoJson) Array() ([]interface{}, error) {
	if j.IsSlice() == false {
		return nil, fmt.Errorf("%v is not array", j.data)
	}

	switch v := j.data.(type) {
	case List:
		return v, nil
	case []interface{}:
		return v, nil
	default:
		return nil, fmt.Errorf("%v is not array", j.data)
	}
}

// RangeMap 遍历kv结构， 传入的函数用于处理遍历。如果这个函数返回false，遍历将立刻结束
func (j *GoJson) RangeMap(f func(key string, val interface{}) bool) error {
	if j.IsMap() == false {
		return fmt.Errorf("%v is not map", j.data)
	}
	switch v := j.data.(type) {
	case Dict:
		for key, val := range v {
			ret := f(key, val)
			if !ret {
				break
			}
		}
	case map[string]interface{}:
		for key, val := range v {
			ret := f(key, val)
			if !ret {
				break
			}
		}
	}
	return nil
}

// RangeSlice 遍历数组结构， 传入的函数用于处理遍历。如果这个函数返回false，遍历将立刻结束
func (j *GoJson) RangeSlice(f func(index int, val interface{}) bool) error {
	if j.IsSlice() == false {
		return fmt.Errorf("%v is not Slice", j.data)
	}
	switch v := j.data.(type) {
	case List:
		for key, val := range v {
			ret := f(key, val)
			if !ret {
				break
			}
		}
	case []interface{}:
		for key, val := range v {
			ret := f(key, val)
			if !ret {
				break
			}
		}
	}
	return nil
}

func handlerVal(val interface{}, cutLongStr bool) interface{} {
	switch valV := val.(type) {
	case Dict:
		return handlerMap(valV, cutLongStr)
	case map[string]interface{}:
		return handlerMap(valV, cutLongStr)
	case List:
		return handlerSlice(valV, cutLongStr)
	case []interface{}:
		return handlerSlice(valV, cutLongStr)
	case string:
		return handlerString(valV, cutLongStr)
	default:
		return valV
	}
}

func handlerMap(js interface{}, cutLongStr bool) Dict {
	ret := NewDict()
	switch v := js.(type) {
	case Dict:
		for key, val := range v {
			ret[key] = handlerVal(val, cutLongStr)
		}
	case map[string]interface{}:
		for key, val := range v {
			ret[key] = handlerVal(val, cutLongStr)
		}
	}
	return ret
}

func handlerSlice(js interface{}, cutLongStr bool) List {
	ret := NewList()
	switch v := js.(type) {
	case List:
		for _, val := range v {
			ret = append(ret, handlerVal(val, cutLongStr))
		}
	case []interface{}:
		for _, val := range v {
			ret = append(ret, handlerVal(val, cutLongStr))
		}
	}
	return ret
}

func handlerString(js string, cutLongStr bool) string {
	if cutLongStr && len(js) > 120 {
		return js[:120] + "......"
	}
	return js
}

// ShortNiceJson 性能差，返回处理过的json，这个json中所有的字符串都被截断成不超过20个字符的数据
func (j *GoJson) ShortNiceJson() *GoJson {
	cutLongStr := true
	if j.IsSlice() {
		return NewJson(handlerSlice(j.data, cutLongStr))
	}
	if j.IsMap() {
		return NewJson(handlerMap(j.data, cutLongStr))
	}
	return NewJson(j.data)
}

// Clone 把这个json对象clone一份，深复制，性能差
func (j *GoJson) Clone() *GoJson {
	cutLongStr := false
	if j.IsSlice() {
		return NewJson(handlerSlice(j.data, cutLongStr))
	}
	if j.IsMap() {
		return NewJson(handlerMap(j.data, cutLongStr))
	}
	return NewJson(j.data)
}

func NewList() List {
	l := make([]interface{}, 0)
	return List(l)
}

type List []interface{}

func (l List) Add(i interface{}) List {
	tp, _ := appendSlice(l, i)
	return tp.(List)
}

func (l *List) String() string {
	result, err := json.Marshal(l)
	if err != nil {
		panic("json to string error" + err.Error())
	}
	return string(result)

}

func NewDict() Dict {
	d := make(map[string]interface{})
	return Dict(d)
}

type Dict map[string]interface{}

func (d *Dict) String() string {
	result, err := json.Marshal(d)
	if err != nil {
		log.Println("json to string error", err)
		return ""
	}
	return string(result)
}

func ToString(obj interface{}) string {
	switch v := obj.(type) {
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", obj)
	}
}

func ToJsonString(obj interface{}) string{
	bytes, _ := json.Marshal(obj)
	// byte数组直接转成string对象，不发生内存copy, benchmark比常规转换性能提升数倍，适合[]byte只读的情况
	return *(*string)(unsafe.Pointer(&bytes))
}

func ToInt(intObj interface{}) (int, error) {
	// 假定int == int64，运行在64位机
	switch v := intObj.(type) {
	case sysjson.Number:
		vint64, err := v.Int64()
		return int(vint64), err
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		if v > math.MaxInt64 {
			info := fmt.Sprintf("ToInt, error, overflowd %v", v)
			return 0, errors.New(info)
		}
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		strv := v
		if strings.Contains(v, ".") {
			strv = strings.Split(v, ".")[0]
		}
		if strv == "" {
			return 0, nil
		}
		if intv, err := strconv.Atoi(strv); err == nil {
			return intv, nil
		}
	}
	return 0, fmt.Errorf("%v cannot convert to int", intObj)
}

func ToFloat64(item interface{}) (float64, error) {
	switch v := item.(type) {
	case sysjson.Number:
		return v.Float64()
	case int, int8, int16, int64, uint, uint8, uint16, uint32, uint64:
		intVal, err := ToInt(item)
		return float64(intVal), err
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case string:
		if floatNum, err := strconv.ParseFloat(v, 64); err == nil {
			return floatNum, nil
		}
	}
	return 0, fmt.Errorf("%v cannot convert to float", item)
}

func ToBool(item interface{}) (bool, error) {

	switch v := item.(type) {
	case bool:
		return v, nil
	default:
		boolValue, err := strconv.ParseBool(ToString(item))
		if err != nil {
			return false, fmt.Errorf("%v cannot convert to bool", item)
		}
		return boolValue, nil
	}
}
