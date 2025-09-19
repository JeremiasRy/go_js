function counter() {
    let count = 0

    return {
        increment: () => count++,
        count: () => count,
    }
}

const c1 = counter()
const c2 = counter()
const c3 = counter()

c1.increment()

c2.increment()
c2.increment()

c3.increment()
c3.increment()
c3.increment()

console.log("c1 count " + c1.count())
console.log("c2 count " + c2.count())
console.log("c3 count " + c3.count())
