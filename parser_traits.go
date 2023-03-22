package smithy

import (
	"strings"

	"github.com/boynton/data"
)

var (
	genericAccepts = []string{"*"}
)

func DefaultTraitVisitors() []TraitVisitor {
	const smithyNamespace = "smithy.api#"

	return []TraitVisitor{
		NewTraitMarker(smithyNamespace,
			"idempotent", "required", "httpLabel", "httpPayload", "readonly", "box",
			"sensitive", "input", "output", "httpResponseCode",
		),

		NewTraitString(smithyNamespace, true, "documentation"),

		NewTraitString(smithyNamespace, false,
			"httpQuery", "httpHeader", "error", "pattern",
			"title", "timestampFormat", "enumValue",
		),

		NewTraitTag(),

		NewTraitInt(smithyNamespace, "httpError"),

		NewTraitWithArgs(smithyNamespace,
			"http", "length", "range", "deprecated", "paginated",
		),

		DeprecatedTrait(NewTraitWithArgs(smithyNamespace, "enum")),

		NewTraitWithLiteral(smithyNamespace, "examples"),

		NewTrait(smithyNamespace, "trait"),

		NewTraitGeneric(),
	}
}

func NewTrait(namespace string, accepts ...string) Trait {
	if !strings.HasSuffix(namespace, "#") {
		namespace += "#"
	}

	return Trait{
		ns:      namespace,
		accepts: accepts,
	}
}

type Trait struct {
	ns      string
	accepts []string
}

func (t Trait) Accepts() []string { return t.accepts }

func (t Trait) Parse(p Parser, name string, traits *data.Object) (*data.Object, error) {
	args, lit, err := p.ParseTraitArgs()
	if err != nil {
		return traits, err
	}
	if lit != nil {
		return WithTrait(traits, t.ns+name, lit), nil
	}
	if args.Length() == 0 {
		return WithTrait(traits, t.ns+name, data.NewObject()), nil
	}
	return WithTrait(traits, t.ns+name, args), nil
}

func NewTraitGeneric() TraitGeneric { return TraitGeneric{} }

type TraitGeneric struct{}

func (t TraitGeneric) Accepts() []string { return genericAccepts }

func (t TraitGeneric) Parse(p Parser, name string, traits *data.Object) (*data.Object, error) {
	args, lit, err := p.ParseTraitArgs()
	if err != nil {
		return traits, err
	}
	tid := p.EnsureNamespaced(name)
	if lit != nil {
		return WithTrait(traits, tid, lit), nil
	}
	return WithTrait(traits, tid, args), nil
}

func NewTraitMarker(namespace string, accepts ...string) TraitMarker {
	return TraitMarker{
		Trait: NewTrait(namespace, accepts...),
	}
}

type TraitMarker struct {
	Trait
}

func (t TraitMarker) Parse(_ Parser, name string, traits *data.Object) (*data.Object, error) {
	return WithTrait(traits, t.ns+name, data.NewObject()), nil
}

func NewTraitString(namespace string, comments bool, accepts ...string) TraitString {
	return TraitString{
		Trait:    NewTrait(namespace, accepts...),
		comments: comments,
	}
}

type TraitString struct {
	Trait
	comments bool
}

func (t TraitString) Parse(p Parser, name string, traits *data.Object) (*data.Object, error) {
	err := p.Expect(OPEN_PAREN)
	if err != nil {
		return traits, err
	}
	s, err := p.ExpectString()
	if err != nil {
		return traits, err
	}
	err = p.Expect(CLOSE_PAREN)
	if err != nil {
		return traits, err
	}

	if t.comments {
		traits, _ = WithCommentTrait(traits, t.ns, s)
		return traits, nil
	}

	return WithTrait(traits, t.ns+name, s), nil
}

func NewTraitInt(namespace string, accepts ...string) TraitInt {
	return TraitInt{
		Trait: NewTrait(namespace, accepts...),
	}
}

type TraitInt struct {
	Trait
}

func (t TraitInt) Parse(p Parser, name string, traits *data.Object) (*data.Object, error) {
	err := p.Expect(OPEN_PAREN)
	if err != nil {
		return traits, err
	}
	n, err := p.ExpectInt()
	if err != nil {
		return traits, err
	}
	err = p.Expect(CLOSE_PAREN)
	if err != nil {
		return traits, err
	}
	return WithTrait(traits, t.ns+name, n), nil
}

func NewTraitTag() TraitTag {
	return TraitTag{
		Trait: NewTrait("smithy.api#tags", "tags"),
	}
}

type TraitTag struct {
	Trait
}

func (t TraitTag) Parse(p Parser, name string, traits *data.Object) (*data.Object, error) {
	_, tags, err := p.ParseTraitArgs()
	return WithTrait(traits, t.ns, tags), err
}

func NewTraitWithArgs(namespace string, accepts ...string) TraitWithArgs {
	return TraitWithArgs{
		Trait: NewTrait(namespace, accepts...),
	}
}

type TraitWithArgs struct {
	Trait
}

func (t TraitWithArgs) Parse(p Parser, name string, traits *data.Object) (*data.Object, error) {
	args, _, err := p.ParseTraitArgs()
	if err != nil {
		return traits, err
	}
	return WithTrait(traits, t.ns+name, args), nil
}

func NewTraitWithLiteral(namespace string, accepts ...string) TraitWithLiteral {
	return TraitWithLiteral{
		Trait: NewTrait(namespace, accepts...),
	}
}

type TraitWithLiteral struct {
	Trait
}

func (t TraitWithLiteral) Parse(p Parser, name string, traits *data.Object) (*data.Object, error) {
	_, lit, err := p.ParseTraitArgs()
	if err != nil {
		return traits, err
	}
	if lit == nil {
		return traits, p.SyntaxError()
	}
	return WithTrait(traits, t.ns+name, lit), nil
}

func DeprecatedTrait(other TraitVisitor) TraitDeprecated {
	return TraitDeprecated{
		other: other,
	}
}

type TraitDeprecated struct {
	other TraitVisitor
}

func (t TraitDeprecated) Accepts() []string { return t.other.Accepts() }

func (t TraitDeprecated) Parse(p Parser, name string, traits *data.Object) (*data.Object, error) {
	p.Warning("Deprecated trait: enum")
	return t.other.Parse(p, name, traits)
}
