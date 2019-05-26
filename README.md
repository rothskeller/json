# github.com/rothskeller/json

github.com/rothskeller/json is a low-level Go language JSON serialization and
deserialization library with a focus on efficiency, output control, and
avoidance of heap allocations.

## Emitting JSON

The `Writer` object emits JSON to the `io.Writer` given as a parameter to
`NewWriter`.  It uses a functional style to avoid heap allocations:

```go
{
    var jw = json.NewWriter(os.Stdout)
    jw.Object(func() {
        jw.Prop("id", 12)
        jw.Prop("nestedObject", func() {
            jw.Object(func() {
                jw.Prop("innerField", "hello")
            })
        })
        jw.Prop("nestedArray", func() {
            jw.Array(func() {
                jw.Int(12)
                jw.String("foo")
            })
        })
    })
    jw.Close()
}
```

This style is very efficient (function calls are cheap, and reflection is not
used), and it allows fine-grained control over what gets emitted.  The library
also offers `NewGzipWriter`, which gzips the JSON output on the fly.

`Writer` has the following methods:

* `Object(f func())`  
  Emits a JSON object.  The supplied function must call `Prop` for each
  property of the object.

* `Prop(name string, value interface{})`  
  Emits a property of a JSON object, with the specified name and value.  The
  value must be `nil`, `string`, `int`, `bool`, `float64`, or a function with
  no parameters and no return value (which is called to produced the value).

* `Array(f func())`  
  Emits a JSON array.  Each `Writer` call within the supplied function generates
  a new value in the array.

* `Null()`  
  `String(s string)`  
  `Int(i int)`  
  `Bool(b bool)`  
  Emit JSON-encoded primitives of the respective types.

* `Raw(s string)`  
  `RawByte(b byte)`  
  Emit a raw string or byte to the JSON output.  Note that the `Writer` will not
  emit a comma after this string or byte; the caller must do so explicitly if
  needed for syntax correctness.

* `Close()`  
  Flushes the output and recycles the internal buffers.  Do not make any calls
  to the `Writer` after calling `Close`.

## Parsing JSON

To parse a JSON stream, call `NewReader` with an input stream.  Then call `Read`
on that reader, and give it a `Handlers` structure telling it how to handle the
JSON that it finds.  Most of the time, you'll use one of the shortcut functions
described below to generate the `Handlers` structure.  The structure has a
function pointer for each type of value that could be seen in the JSON.  Those
types with non-nil function pointers are the ones that are allowed; the
corresponding function will be called when those types are parsed.

* `Null func()`
* `Int func(i int)`
* `Float func(f float64)`
* `String func(s string)`
* `Time func(t time.Time)`
* `Bool func(b bool)`
* `Object func(key string) Handlers`
* `Array func() Handlers`

When an integer is parsed, the `Int` function pointer will be used if provided,
otherwise the `Float` function pointer will be used.  When a timestamp is
parsed, the `Time` function pointer will be used if provided, otherwise the
`String` function pointer will be used.

When an object is parsed, the `Object` function pointer will be called for each
key in the object, and it should return a separate `Handlers` structure
describing how to parse the value of that key.  If it returns a `Handlers`
structure with no function pointers set, the key will be reported as
unrecognized.  If it returns a `Handlers` structure with the `Ignore` flag set,
the key and its will be ignored (recursively).

When an array is parsed, the `Array` function pointer will be called for each
element in the array, and it should return a separate `Handlers` structure
describing how to parse the value of that element.

You generally only need to write a `Handlers` structure directly when you're
dealing with polymorphic values.  If you know in advance the JSON types you
expect, there are shortcut functions to supply the `Handlers` structure for you:

* `RejectHandler()`
* `IgnoreHandler()`
* `NullHandler()`
* `IntHandler(func(int))`
* `FloatHandler(func(float64))`
* `StringHandler(func(string))`
* `TimeHandler(func(time.Time))`
* `BoolHandler(func(bool))`
* `ObjectHandler(func(string) Handlers)`
* `ArrayHandler(func() Handlers)`
* `IntNullHandler(func(int))`
* `FloatNullHandler(func(float64))`
* `StringNullHandler(func(string))`
* `TimeNullHandler(func(time.Time))`
* `BoolNullHandler(func(bool))`
* `ObjectNullHandler(func(string) Handlers)`
* `ArrayNullHandler(func() Handlers)`

The `RejectHandler` rejects any value; it's normally used to reject unknown keys
of objects.  The `IgnoreHandler` ignores any value, recursively; it's normally
used to ignore unknown keys of objects.  The `xxxNullHandler` functions accept
either the designated type or `null`.  For `IntNullHandler`, `FloatNullHandler`,
`StringNullHandler`, `TimeNullHandler`, and `BoolNullHandler`, if a `null` is
seen, the supplied handler is called with the zero value of the type.  For
`ObjectNullHandler` and `ArrayNullHandler`, if a `null` is seen, the supplied
handler is never called.

In all cases, a handler function can abort the parsing by passing an error
message to the `Raise` method of the reader.
