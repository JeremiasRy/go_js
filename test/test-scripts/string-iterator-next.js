const it = "a😀"[Symbol.iterator]();
console.log(JSON.stringify(it.next()));
console.log(JSON.stringify(it.next()));
console.log(JSON.stringify(it.next()));
