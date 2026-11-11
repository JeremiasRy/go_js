// M5: Private fields/methods/accessors enforce instance and static brands.
class Secret {
    #value = 1;
    static #staticValue = 7;
    #add(amount) { this.#value += amount; }
    get #doubled() { return this.#value * 2; }
    set #doubled(next) { this.#value = next / 2; }
    read() { return this.#value; }
    increment() { this.#add(2); return this.#doubled; }
    assign(next) { this.#doubled = next; }
    static has(value) { return #value in value; }
    static readStatic() { return this.#staticValue; }
}
const first = new Secret();
const second = new Secret();
console.log("private", first.increment(), first.read(), second.read());
first.assign(10);
first["#value"] = 99;
console.log("hidden", first.read(), first["#value"], Object.keys(second).length, Object.getOwnPropertyNames(second).length);
console.log("brands", Secret.has(first), Secret.has({}), Secret.readStatic());
try { Secret.has(1); } catch (error) { console.log("brand-primitive", error.name); }
try { first.read.call({}); } catch (error) { console.log("wrong-receiver", error.name); }
class Child extends Secret {}
console.log("derived-brand", Secret.has(new Child()));
try { Child.readStatic(); } catch (error) { console.log("static-brand", error.name); }
