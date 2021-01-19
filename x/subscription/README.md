# Implementation nodes

## CronSpec

### `CronValue` enum

In specification, `CronValue` is a enum:`Any | Range(start, end, step) | Value(v)`, golang don't have native supported enum, it's emulated with interface.

First of all, the data type needs to be defined in protobuf, `CronValue` is expressed as oneof syntax:

```protobuf
message CronItem {
    oneof spec {
        uint32 value = 1;
        CronRange range = 2;
    }
}

message CronRange {
    uint32 start = 1;
    uint32 stop = 2;
    uint32 step = 3;
}
```

The case of `Any` is represented as `nil`.

Protobuf will generate golang code like this:

```golang
type isCronItem_Spec interface {
	isCronItem_Spec()
	MarshalTo([]byte) (int, error)
	Size() int
}

type CronItem_Value struct {
	Value uint32 `protobuf:"varint,1,opt,name=value,proto3,oneof" json:"value,omitempty"`
}
type CronItem_Range struct {
	Range *CronRange `protobuf:"bytes,2,opt,name=range,proto3,oneof" json:"range,omitempty"`
}

...
```

In logic code, pattern matching can be written as:

```golang
switch spec := item.GetSpec().(type) {
	case nil:
		// Any
	case *CronItem_Value:
    // Value(v)
	case *CronItem_Range:
    // Range(start, end, step)
	default:
		panic("impossible")
	}
```

### Compile `CronSpec`

`CronSpec` specifies valid values of different components of datetime, the valid values get compiled into bitsets, so it's more efficient for query and iterating.