function counter() {
    let count = 0

    return {
        increment: () => count++,
        count: () => count,
    }
}

const c1 = counter()

c1.increment()
c1.increment()
c1.increment()

console.log(c1.count())

const c2 = counter()

console.log(c2.count())

c2.increment()
c2.increment()
c2.increment()
c2.increment()
c2.increment()
c2.increment()
c2.increment()
c2.increment()
c2.increment()
c2.increment()
c2.increment()
c2.increment()

console.log(c2.count())
console.log(c1.count())

c1.increment()
c1.increment()
c1.increment()

console.log(c1.count())

