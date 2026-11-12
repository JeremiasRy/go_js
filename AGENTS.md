# Goal

Make all the tests pass in `./test`, run the full test suite with `./test.sh`.

# Recommended approach

Take one of the failing tests from `./test/failures` and run it with `-debug` flag:

`cd runtime`

`go run main.go -debug go run ./main.go -debug ../test/test-scripts/<test-file>.js`

youll get the AST, Bytecode and execution stqack, should help with debugging.

# Workflow

Make a commit once you have fixed a test case, i.e. 1 commit per fixed test case, write a short description on what was fixed. The commit should contain the implementation and moving the possible test `.js` file from failures to the passing test suite.

# Comments

Do not write comments to source code, only if absolutely necessary
