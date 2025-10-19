
function createCircularReference() {
    let obj1 = {};
    let obj2 = {};

    obj1.ref = obj2;
    obj2.ref = obj1;

    console.log("Circular reference created within function scope.");
    console.log(obj1)
    console.log(obj2)
}
let i = 0
while (i < 10) {
    createCircularReference();
    i++
}

