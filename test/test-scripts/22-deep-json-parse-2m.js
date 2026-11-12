const text = "[".repeat(2000000) + "]".repeat(2000000);
console.log(JSON.parse(text).length);
