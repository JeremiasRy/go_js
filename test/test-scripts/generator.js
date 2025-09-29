function* numberGenerator() {
    let state = 100
    console.log(`travelling ${state}`)
    yield 1;
    state += state
    console.log(`travelling ${state}`)
    yield 2;
    state += state
    console.log(`travelling ${state}`)
    yield 3;
}

const gen = numberGenerator();
let n = gen.next()
console.log(n.value);
console.log(n.done);

n = gen.next()
console.log(n.value);
console.log(n.done);

n = gen.next()
console.log(n.value);
console.log(n.done);

n = gen.next()
console.log(n.value);
console.log(n.done);

