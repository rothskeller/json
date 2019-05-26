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
