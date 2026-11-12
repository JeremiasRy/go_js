const { proxy, revoke } = Proxy.revocable({ a: 1 }, {});
console.log(proxy.a);
revoke();
try { console.log(proxy.a); } catch (e) { console.log(e.name); }
