$version: "1.0"

metadata foo = "bar"

namespace smithy.example

use smithy.other.namespace#MyString

structure MyStructure {
    @required
    foo: MyString,
}
