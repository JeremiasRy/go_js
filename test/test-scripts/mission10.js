let greeting = "hello";
if (true) {
    let inner = "world";
    console.log(greeting + " " + inner);
}
try {
    console.log(inner);
} catch (e) {
    console.log("Error: 'inner' is not defined outside its block");
}