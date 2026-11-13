function build(n) {
  let text = "a";
  for (let i = 0; i < n; i++) {
    text = text + i;
    if (text.length > 1000) {
      break;
    }
  }
  return text;
}

function templates(n) {
  let text = "";
  for (let i = 0; i < n; i++) {
    text = `${text}${i},`;
  }
  return text;
}

for (let i = 0; i < 2000; i++) {
  build(3);
  templates(3);
}

const built = build(20);
console.log(built);
console.log(built.length);
console.log(templates(5));
