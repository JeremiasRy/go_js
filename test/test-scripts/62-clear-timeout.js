const id = setTimeout(() => console.log("ran"), 0);
clearTimeout(id);
console.log("cleared");
