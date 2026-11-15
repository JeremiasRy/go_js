console.log('sync start');
setTimeout(() => {
  console.log('timer1');
  Promise.resolve().then(() => console.log('micro in timer'));
  console.log('timer1 end');
}, 0);
Promise.resolve().then(() => console.log('micro1')).then(() => console.log('micro2'));
queueMicrotask(() => console.log('qm'));
console.log('sync end');
