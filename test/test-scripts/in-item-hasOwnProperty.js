const parent = { a: 1 };
const child = Object.create(parent);
child.b = 2;

let result = '';
for (const key in child) {
    if (child.hasOwnProperty(key)) {
        result += key;
    }
}
console.log(result);