## GO_JS

Not that he world needs another javaScript runtime, it's an educational project for me.

**In short** (the "cool" stuff)

- Event loop (setTimeout, async/await, micro/macro task queues)
- [JIT](https://en.wikipedia.org/wiki/Just-in-time_compilation) compilation, pretty limited at the moment, but can handle recursive fibonacci for exanple. Check [jit-compiler](runtime/jit/compiler.go).
- Closures
- Garbage collection (not a language feature, but can be a tricky thing to implement)

## How to run
Requires go version `1.22.5`
1. `git clone` the repo
2. `cd /runtime`
3. `go run main.go <your_javascript_file.js>` use `--debug` flag to see all the fun stuff

Test suite can be run with docker by running `./test.sh` in the root of the repo.

## Test runner (Docker)
[`test.sh`](/test.sh)

Scripts in [test-scripts](test/test-scripts/) are executed against NodeJS, and the output then compared to my runtimes output of executing the same script.
If the outputs match the test case is considered succesful.

It also runs scripts in [benchmarks](test/benchmark-scripts) folder to measure the performance against NodeJS. 

Adding files to the folders will add them to the test-suites.

## Language features handled

A good representation of what is implemented can be found by going to [test-scripts](test/test-scripts/) AND [benchmarks](test/benchmark-scripts)
It can also handle a real program as shown in an [Advent Of Code](/test/aoc/solution.js) solution.

I skipped `var` for the runtime implementation, also some quirks of javaScript are skipped, i.e `if(a = someFunc()) {}` type of stuff.

### Flow of execution / Design

1. [Parse](runtime/parser/parser.go) javascript file into an [Abstract Syntax Tree](https://en.wikipedia.org/wiki/Abstract_syntax_tree)
2. [Pre-pass](runtime/compiler/prepass.go) the syntax tree to define all variables to symboltables and figure out if anything is captured by a closure etc.
3. [Compile](runtime/compiler/compiler.go) the syntax tree into [bytecode](https://en.wikipedia.org/wiki/Bytecode)
4. [Execute](runtime/vm/vm.go) the bytecode

### Things to do
As in any projects they really are never done, and things can be improved endlessly. And in the case of this project, I can't claim *full javaScript runtime* (I know sad right?)

**Notable things to improve/add**
1. Proper error handling for rejected promises
2. Support `var`, parser already parses these so wouldn't be too much work to add it in
4. Http requests with `fetch()`
5. A http server? Would just be a wrapper for go's `net/http`

### Notes

The `parser` package is a port of [acorn](https://github.com/acornjs/acorn) to go. 
