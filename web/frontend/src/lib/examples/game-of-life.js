const WIDTH = 44;
const HEIGHT = 20;
const GENERATIONS = 12;

let cells = [];

for (let y = 0; y < HEIGHT; y++) {
    const row = [];
    for (let x = 0; x < WIDTH; x++) {
        row.push(0);
    }
    cells.push(row);
}

const seed = [
    [20, 8],
    [21, 8],
    [19, 9],
    [20, 9],
    [20, 10],
];

for (const [x, y] of seed) {
    cells[y][x] = 1;
}

function neighbours(grid, x, y) {
    let count = 0;

    for (let dy = -1; dy <= 1; dy++) {
        for (let dx = -1; dx <= 1; dx++) {
            if (dx !== 0 || dy !== 0) {
                const nx = (x + dx + WIDTH) % WIDTH;
                const ny = (y + dy + HEIGHT) % HEIGHT;
                count += grid[ny][nx];
            }
        }
    }

    return count;
}

for (let generation = 0; generation < GENERATIONS; generation++) {
    console.log(`Generation ${generation}`);

    for (let y = 0; y < HEIGHT; y++) {
        let row = "";

        for (let x = 0; x < WIDTH; x++) {
            row += cells[y][x] === 1 ? "██" : "  ";
        }

        console.log(row);
    }

    const next = [];

    for (let y = 0; y < HEIGHT; y++) {
        const row = [];

        for (let x = 0; x < WIDTH; x++) {
            const count = neighbours(cells, x, y);
            let nextValue = 0;

            if (cells[y][x] === 1) {
                if (count === 2 || count === 3) {
                    nextValue = 1;
                }
            } else if (count === 3) {
                nextValue = 1;
            }

            row.push(nextValue);
        }

        next.push(row);
    }

    cells = next;
}
