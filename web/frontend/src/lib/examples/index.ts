import mandelbrot from "./mandelbrot.js?raw";
import gameOfLife from "./game-of-life.js?raw";
import nQueens from "./n-queens.js?raw";
import fibonacci from "./fibonacci.js?raw";

export type CodeExample = {
    id: string;
    title: string;
    description: string;
    source: string;
};

export const examples: CodeExample[] = [
    {
        id: "mandelbrot",
        title: "Mandelbrot explorer",
        description:
            "Render the escape-time fractal as ASCII art with nested loops.",
        source: mandelbrot,
    },
    {
        id: "game-of-life",
        title: "Conway's Game of Life",
        description:
            "Watch a five-cell seed evolve across a wrapping 44 × 20 grid.",
        source: gameOfLife,
    },
    {
        id: "n-queens",
        title: "Eight queens",
        description:
            "Backtracking search finds every solution to the classic puzzle.",
        source: nQueens,
    },
    {
        id: "fibonacci",
        title: "Fibonacci benchmark",
        description: "Measure recursive fib(40) in the runtime.",
        source: fibonacci,
    },
];

export const defaultExample = fibonacci;
