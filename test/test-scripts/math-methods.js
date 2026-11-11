// M3: Deterministic Math operations, conversion, infinities, NaN, and -0.
console.log("rounding", Math.floor(-1.2), Math.ceil(-1.2), Math.trunc(-1.2), Math.round(1.5), Math.round(-1.5));
console.log("negative-zero", Object.is(Math.round(-0.5), -0), Object.is(Math.trunc(-0.2), -0), Object.is(Math.ceil(-0.2), -0));
console.log("abs-sign", Math.abs(-4), Object.is(Math.abs(-0), 0), Math.sign(-3), Math.sign(3), Object.is(Math.sign(-0), -0));
console.log("min-max", Math.min(3, 1, 2), Math.max(3, 1, 2), Math.min(), Math.max());
console.log("zero-order", Object.is(Math.min(0, -0), -0), Object.is(Math.max(-0, 0), 0));
console.log("powers", Math.pow(2, 10), Math.sqrt(81), Math.sqrt(Infinity), Math.pow(0, 0));
console.log("nan", Number.isNaN(Math.sqrt(-1)), Number.isNaN(Math.min(1, NaN)), Number.isNaN(Math.pow(-1, Infinity)));
console.log("conversion", Math.abs("-3"), Math.floor(null), Math.max("2", true));
