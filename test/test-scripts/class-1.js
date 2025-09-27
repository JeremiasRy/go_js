let a = "jau"

class Person {
    constructor(name) {
        this.name = name;
    }
    greet() {
        console.log(`Hello, my name is ${this.name}. I'm a ${this.type}`);
    }

    calculate() {
        let a = 2
        let b = 5
        let result = b / a
        console.log(`Intermediate calculation from ${this.name} ${result}`)
    }

    type = a
    type2 = "jausers"
}

const person = new Person("Eve");
person.greet();
person.calculate()

const person2 = new Person("James");
person2.greet();
person2.calculate()
