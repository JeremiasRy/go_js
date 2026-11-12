const f = async function () { return 1; };
f().then(v => console.log(v));
console.log("sync");
