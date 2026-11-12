function tag(strings, ...values) {
  return strings[0] + values.length;
}
console.log(tag`a${1}b${2}c`);
