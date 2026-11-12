// Expected: parse-time SyntaxError for an undeclared private name.
class Invalid {
    read() { return this.#missing; }
}
