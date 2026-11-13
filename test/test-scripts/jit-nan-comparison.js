function le(n) {
  if (n <= 1) return 1;
  return 0;
}

function lt(n) {
  if (n < 1) return 1;
  return 0;
}

function ge(n) {
  if (n >= 1) return 1;
  return 0;
}

function gt(n) {
  if (n > 1) return 1;
  return 0;
}

let sum = 0;
for (let i = 0; i < 10000; i++) {
  sum = sum + le(i) + lt(i) + ge(i) + gt(i);
}

console.log(le(NaN), lt(NaN), ge(NaN), gt(NaN));
console.log(le(-Infinity), lt(Infinity), ge(-Infinity), gt(Infinity));
console.log(sum);
