# Goal

Make all the tests pass in `./test`, run the full test suite with `./test.sh`. It'll blurp out a bunch of text and tells you which tests have passed and what haven't.

# Recommended approach

Take one of the failing tests and run it with `-debug` flag:

`cd runtime`

`go run main.go -debug go run ./main.go -debug ../test/test-scripts/<test-file>.js`

youll get the AST, Bytecode and execution stqack, should help with debugging.

# Workflow

Make a commit once you have fixed a test case, i.e. 1 commit per fixed test case, write a short description on what was fixed.

# Comments

Donot write comments to source code, only if absolutely necessary
