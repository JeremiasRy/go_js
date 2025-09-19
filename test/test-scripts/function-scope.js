const obj = {
    name: "John",
    greet: function () {
        console.log("Method call: " + this.name);
    }
};

const greetFunc = obj.greet;

obj.greet();
greetFunc();