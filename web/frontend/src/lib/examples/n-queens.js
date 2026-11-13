const SIZE = 8;
const queens = [];
let solutions = 0;
let firstSolution = null;

for (let i = 0; i < SIZE; i++) {
    queens.push(-1);
}

function canPlace(row, column) {
    for (let previousRow = 0; previousRow < row; previousRow++) {
        const previousColumn = queens[previousRow];

        if (previousColumn === column) {
            return false;
        }

        if (Math.abs(previousColumn - column) === row - previousRow) {
            return false;
        }
    }

    return true;
}

function solve(row) {
    if (row === SIZE) {
        solutions++;

        if (firstSolution === null) {
            firstSolution = queens.slice();
        }

        return;
    }

    for (let column = 0; column < SIZE; column++) {
        if (!canPlace(row, column)) {
            continue;
        }

        queens[row] = column;
        solve(row + 1);
        queens[row] = -1;
    }
}

solve(0);

console.log(`Found ${solutions} solutions to the ${SIZE}-queens problem.`);
console.log("First solution:");

for (let row = 0; row < SIZE; row++) {
    let line = "";

    for (let column = 0; column < SIZE; column++) {
        line += firstSolution[row] === column ? "♛ " : "· ";
    }

    console.log(line);
}
