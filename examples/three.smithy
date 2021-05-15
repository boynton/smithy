$version: "1.0"

namespace smithy.example

structure MyStructure {
    @required
    foo: smithy.other#MyString,
	bar: String,
}

union MyUnion {
   blah: String,
   bar: smithy.other#String,
   foo: MyStructure
}
