function closureAdderClass(amount) {
    let a = 0

    class Calculator {
        constructor(amount) {
            this.amount = amount
        }

        calculate() {
            a += this.amount
        }

        result() {
            return a
        }
    }

    return new Calculator(amount)
}

const add5 = closureAdderClass(5)
const add367 = closureAdderClass(367)

add5.calculate()
add5.calculate()

console.log(add5.result())

add367.calculate()
add367.calculate()
add367.calculate()
add367.calculate()

console.log(add367.result())