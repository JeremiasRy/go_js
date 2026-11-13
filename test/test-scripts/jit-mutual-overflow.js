function down(n) {
  if (n === 0) return 0;
  return up(n - 1);
}

function up(n) {
  if (n === 0) return 0;
  return down(n - 1);
}

try {
  down(100000);
} catch (e) {
  console.log(e.name);
}
console.log(down(100));
