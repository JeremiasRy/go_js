class Animal {
    constructor(name) {
        this.name = name;
    }
}

const dog = new Animal('dog');

console.log(dog.speak)

Animal.prototype.speak = function () {
    console.log(`${this.name} makes noise`)
}

dog.speak();