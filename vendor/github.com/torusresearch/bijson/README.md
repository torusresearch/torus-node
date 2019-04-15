# bijson: optimized standard library JSON for Go

Clone of `fastjson` by intel-go,`bijson` has the same API as json from standard library `encoding/json`. 

## Getting Started
```
$go get github.com/YZhenY/bijson
```


##Example
```Go
import (
    "github.com/YZhenY/bijson"
    "fmt"
)

func main() {
    var jsonBlob = []byte(`[
	{"Name": "Platypus", "Order": "Monotremata"},
	{"Name": "Quoll",    "Order": "Dasyuromorphia"}
    ]`)
    type Animal struct {
	Name  string
	Order string
    }
    var animals []Animal
    err := bijson.Unmarshal(jsonBlob, &animals)
    if err != nil {
	fmt.Println("error:", err)
    }
    fmt.Printf("%+v", animals)
    // Output:
    // [{Name:Platypus Order:Monotremata} {Name:Quoll Order:Dasyuromorphia}]
}
```
##API
API is the same as encoding/json
[GoDoc](https://golang.org/pkg/encoding/json/#Unmarshal)
