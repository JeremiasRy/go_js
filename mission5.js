

const myHash = {
    h: "hello",
    v: 12,
    hello: (h) => h,
    value: function (v) { return v }
}

console.log(myHash.hello(myHash.h))
console.log(myHash.value(myHash.v))