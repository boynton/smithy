$version: "1.0"

namespace smithy.example

@trait
structure Beta {
}

@Beta
structure Foo {
    @required
    descr: String,
}

@Beta
string Blah

@trait(selector: "string", conflicts: ["Beta"]) 
structure structuredTrait {
    @required
    lorem: StringShape,

    @required
    ipsum: StringShape,

    dolor: StringShape,
}

/// Apply the "beta" trait to the "foo" member.
structure MyShape {
    @required
    @Beta
    foo: StringShape,
}

/// Apply the structuredTrait to the string.
@structuredTrait(
    lorem: "This is a custom trait!",
    ipsum: "lorem and ipsum are both required values.")
string StringShape

