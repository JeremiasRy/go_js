function sum(...numbers) {
    return numbers.reduce((total, num) => total + num, 0);
}

const arr = [1, 2, 3];
console.log(sum(...arr));