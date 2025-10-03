function sum(start, ...numbers) {
    return numbers.reduce((total, num) => total + num, start);
}

const arr = [1, 2, 3];
console.log(sum(1, ...arr));
console.log(sum(5, 7, 4, 1, 2));