// Package apischema lists the fields of some of the API's resources, and the
// resource that each field refers to, judged by the field's name and shape.
package apischema

type Resource struct {
	// Fields are the fields of the create and update bodies and of the retrieve
	// response, keyed by path, such as "chapters[].title". "[]" stands for every
	// element of a list. The values of a map have no fields here.
	Fields map[string]Field
}

type Field struct {
	// A field with neither InCreate nor InResponse is only in the update body.
	InCreate, InResponse bool
	// List is set if the value can be a list.
	List bool
	// References are the ways found in the create and update bodies in which the
	// field, or each element of it, refers to a resource.
	References []Reference
}

type Reference struct {
	// Resource is the name of that resource, written like the keys of Resources.
	// That resource need not be in Resources.
	Resource string
	// Type and IDField are set if the value is an object: Type is the value of its
	// "type" property, and IDField the property that holds the ID. Otherwise the
	// value is the ID itself.
	Type, IDField string
}
