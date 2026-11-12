Promise.resolve(1).finally(() => console.log("fin")).then(v => console.log("value", v));
Promise.reject("x").finally(() => console.log("fin2")).catch(e => console.log("reason", e));
Promise.resolve(1).finally(() => { throw "y"; }).catch(e => console.log("throw", e));
