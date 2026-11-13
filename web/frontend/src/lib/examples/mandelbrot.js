const WIDTH = 76;
const HEIGHT = 30;
const SHADES = " .:-=+*#%@";

for (let y = 0; y < HEIGHT; y++) {
    let row = "";

    for (let x = 0; x < WIDTH; x++) {
        const real = (x / WIDTH) * 3.5 - 2.5;
        const imag = (y / HEIGHT) * 2 - 1;
        let zr = 0;
        let zi = 0;
        let iteration = 0;

        while (
            iteration < 80 &&
            zr * zr + zi * zi <= 4
        ) {
            const next = zr * zr - zi * zi + real;
            zi = 2 * zr * zi + imag;
            zr = next;
            iteration++;
        }

        const index = Math.min(
            SHADES.length - 1,
            Math.floor((iteration / 80) * SHADES.length),
        );
        row += SHADES[index];
    }

    console.log(row);
}
