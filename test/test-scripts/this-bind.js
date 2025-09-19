const person = {
    name: "Bob",
    sayName: function () {
        setTimeout(() => {
            console.log(this.name);
        }, 10);
    }
};

person.sayName();