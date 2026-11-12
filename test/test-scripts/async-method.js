const o = { async m() { return 1; } };
o.m().then(v => console.log(v));
class A { async m() { return 2; } }
new A().m().then(v => console.log(v));
