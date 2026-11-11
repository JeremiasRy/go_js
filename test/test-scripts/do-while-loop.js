// M1: Post-test loops, including continue's jump to the condition.
let count = 0;
do { count++; } while (false);
console.log("once", count);
let tests = 0;
let trace = "";
do {
    count++;
    if (count === 2) continue;
    trace += count;
} while (++tests < 3);
console.log("continue", count, tests, trace);
tests = 0;
do { break; } while (++tests);
console.log("break-skips-condition", tests);
let remaining = 2;
do remaining--; while (remaining > 0);
console.log("single-statement", remaining);
