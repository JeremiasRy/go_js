// M6: Revocation is idempotent and disables all forwarded internal operations.
const record = Proxy.revocable({ value: 1 }, {});
console.log("before", record.proxy.value, typeof record.revoke);
console.log("revoke", record.revoke(), record.revoke(), typeof record.proxy);
for (const operation of [() => record.proxy.value, () => "value" in record.proxy, () => Object.keys(record.proxy), () => Object.getPrototypeOf(record.proxy)]) {
    try { operation(); } catch (error) { console.log("revoked-object", error.name); }
}
const callable = Proxy.revocable(function Item() { this.value = 2; }, {});
console.log("constructed-before", new callable.proxy().value);
callable.revoke();
console.log("callable-type", typeof callable.proxy);
for (const operation of [() => callable.proxy(), () => new callable.proxy()]) {
    try { operation(); } catch (error) { console.log("revoked-call", error.name); }
}
const array = Proxy.revocable([], {});
console.log("array-before", Array.isArray(array.proxy));
array.revoke();
try { Array.isArray(array.proxy); } catch (error) { console.log("array-after", error.name); }
