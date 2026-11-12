const f = async () => { console.log("start"); await null; console.log("end"); };
f();
console.log("sync");
