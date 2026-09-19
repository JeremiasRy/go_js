package chunk

const (
	OP_CONSTANT uint8 = iota
	OP_POP
	OP_PUSH_CURRENT
	OP_ARG_START
	OP_B_AND
	OP_B_XOR
	OP_B_OR
	OP_ADD
	OP_SUBTRACT
	OP_MULTIPLY
	OP_DIVIDE
	OP_EXPONENTIATION
	OP_MODULO
	OP_NILL
	OP_UNDEFINED
	OP_EQUALS
	OP_STRICT_EQUALS
	OP_STRICT_NOT_EQUALS
	OP_LESS_THAN_EQUAL
	OP_LESS_THAN
	OP_GREATER_THAN_EQUAL
	OP_GREATER_THAN
	OP_LOGICAL_OR
	OP_LOGICAL_AND
	OP_DEFINE_LOCAL
	OP_GET_LOCAL
	OP_SET_LOCAL
	OP_POP_LOCAL
	OP_DEFINE_GLOBAL
	OP_GET_GLOBAL
	OP_SET_GLOBAL
	OP_CREATE_HEAP_SCOPE
	OP_DEFINE_HEAP_VAR
	OP_SET_HEAP_VAR
	OP_GET_HEAP_VAR
	OP_CALL // op <arguments len> <called with spread 0 false 1 true>
	OP_RETURN
	OP_JUMP_IF_FALSE
	OP_JUMP_IF_TRUE
	OP_JUMP
	OP_CREATE_OBJECT
	OP_SET_OBJECT_MEMBER
	OP_GET_OBJECT_MEMBER
	OP_CREATE_ARRAY
	OP_PUSH_ELEMENT
	OP_PUSH_UNDEFINED
	OP_GET_ITERATOR // op <type>; 0 -> for..of (i.e values), 1 -> for..in (i.e keys/indexes)
	OP_ITERATOR_NEXT
	OP_ITERATOR_CURRENT
	OP_TRY_BLOCK_START
	OP_TRY_BLOCK_END
	OP_THROW
	OP_NEW
	OP_AWAIT
	OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS // op <amount> <...slot>; i.e OP_DEFINE_HEAP_VAR_FROM_PARAMS 2 0 3
	OP_NEGATE
	OP_CREATE_CLASS_START //  pops name from stack, creates empty ClassObj and prototype objects and pushes them to stack [ObjClass, Prototype]
	OP_CREATE_CLASS_END
	OP_PUSH_METHOD
	OP_PUSH_PROPERTY
	OP_THIS
	OP_YIELD
	OP_IMPORT
	OP_EXPORT
	OP_IN
	OP_NOT
	OP_NULL_COALESHING
	OP_SPREAD
	OP_CREATE_REST_OBJECT // op <len of exluded> <...excluded constant slot>
	OP_SET_FROM_SPREAD
)

var OpNames = map[uint8]string{
	OP_CONSTANT:                        "OP_CONSTANT",
	OP_POP:                             "OP_POP",
	OP_PUSH_CURRENT:                    "OP_PUSH_CURRENT",
	OP_ADD:                             "OP_ADD",
	OP_SUBTRACT:                        "OP_SUBTRACT",
	OP_MULTIPLY:                        "OP_MULTIPLY",
	OP_DIVIDE:                          "OP_DIVIDE",
	OP_MODULO:                          "OP_MODULO",
	OP_EXPONENTIATION:                  "OP_EXPONENTIATION",
	OP_EQUALS:                          "OP_EQUALS",
	OP_STRICT_EQUALS:                   "OP_STRICT_EQUALS",
	OP_STRICT_NOT_EQUALS:               "OP_STRICT_NOT_EQUALS",
	OP_LESS_THAN_EQUAL:                 "OP_LESS_THAN_EQUAL",
	OP_LESS_THAN:                       "OP_LESS_THAN",
	OP_GREATER_THAN_EQUAL:              "OP_GREATER_THAN_EQUAL",
	OP_GREATER_THAN:                    "OP_GREATER_THAN",
	OP_LOGICAL_AND:                     "OP_LOGICAL_AND",
	OP_LOGICAL_OR:                      "OP_LOGICAL_OR",
	OP_DEFINE_LOCAL:                    "OP_DEFINE_LOCAL",
	OP_GET_LOCAL:                       "OP_GET_LOCAL",
	OP_SET_LOCAL:                       "OP_SET_LOCAL",
	OP_POP_LOCAL:                       "OP_POP_LOCAL",
	OP_DEFINE_GLOBAL:                   "OP_DEFINE_GLOBAL",
	OP_GET_GLOBAL:                      "OP_GET_GLOBAL",
	OP_SET_GLOBAL:                      "OP_SET_GLOBAL",
	OP_CREATE_HEAP_SCOPE:               "OP_CREATE_HEAP_SCOPE",
	OP_DEFINE_HEAP_VAR:                 "OP_DEFINE_HEAP_VAR",
	OP_GET_HEAP_VAR:                    "OP_GET_HEAP_VAR",
	OP_SET_HEAP_VAR:                    "OP_SET_HEAP_VAR",
	OP_CALL:                            "OP_CALL",
	OP_RETURN:                          "OP_RETURN",
	OP_JUMP_IF_FALSE:                   "OP_JUMP_IF_FALSE",
	OP_JUMP_IF_TRUE:                    "OP_JUMP_IF_TRUE",
	OP_JUMP:                            "OP_JUMP",
	OP_CREATE_OBJECT:                   "OP_CREATE_OBJECT",
	OP_SET_OBJECT_MEMBER:               "OP_SET_OBJECT_MEMBER",
	OP_GET_OBJECT_MEMBER:               "OP_GET_OBJECT_MEMBER",
	OP_CREATE_ARRAY:                    "OP_CREATE_ARRAY",
	OP_PUSH_ELEMENT:                    "OP_PUSH_ELEMENT",
	OP_PUSH_UNDEFINED:                  "OP_PUSH_UNDEFINED",
	OP_GET_ITERATOR:                    "OP_GET_ITERATOR",
	OP_ITERATOR_NEXT:                   "OP_ITERATOR_NEXT",
	OP_ITERATOR_CURRENT:                "OP_ITERATOR_CURRENT",
	OP_TRY_BLOCK_START:                 "OP_TRY_BLOCK_START",
	OP_TRY_BLOCK_END:                   "OP_TRY_BLOCK_END",
	OP_THROW:                           "OP_THROW",
	OP_NEW:                             "OP_NEW",
	OP_AWAIT:                           "OP_AWAIT",
	OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS: "OP_DEFINE_HEAP_VARS_FROM_ARGUMENTS",
	OP_NEGATE:                          "OP_NEGATE",
	OP_CREATE_CLASS_START:              "OP_CREATE_CLASS_START",
	OP_CREATE_CLASS_END:                "OP_CREATE_CLASS_END",
	OP_PUSH_METHOD:                     "OP_PUSH_METHOD",
	OP_PUSH_PROPERTY:                   "OP_PUSH_PROPERTY",
	OP_THIS:                            "OP_THIS",
	OP_YIELD:                           "OP_YIELD",
	OP_IMPORT:                          "OP_IMPORT",
	OP_EXPORT:                          "OP_EXPORT",
	OP_IN:                              "OP_IN",
	OP_NOT:                             "OP_NOT",
	OP_NULL_COALESHING:                 "OP_NULL_COALESHING",
	OP_SPREAD:                          "OP_SPREAD",
	OP_CREATE_REST_OBJECT:              "OP_CREATE_REST_OBJECT",
	OP_SET_FROM_SPREAD:                 "OP_SET_FROM_SPREAD",
	OP_ARG_START:                       "OP_ARG_START",
	OP_B_AND:                           "OP_B_AND",
	OP_B_OR:                            "OP_B_OR",
	OP_B_XOR:                           "OP_B_XOR",
}
