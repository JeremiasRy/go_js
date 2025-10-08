import { input } from "./input.js"

/**
 * @type {Map<string, number>}
 */
const WIRES = new Map()
const XOR = "XOR"
const OR = "OR"
const AND = "AND"

const [init, program] = input.split("\n\n")

for (const wire of init.split("\n")) {
    const [key, value] = wire.split(": ")
    WIRES.set(key, parseInt(value))
}

/**
 * @type {string[][]}
 */
const commands = []

for (const command of program.split("\n")) {
    if (command.length === 0) {
        continue
    }
    const [src1, oper, src2, _, dst] = command.split(" ")
    commands.push([src1, oper, src2, dst])
}

while (commands.length > 0) {
    const command = commands.shift()

    if (applyCommand(...command)) {
        continue
    } else {
        commands.push(command)
    }
}

const Z_WIRES = []

for (const key of WIRES.keys()) {
    if (key.startsWith('z')) {
        Z_WIRES[parseInt(key.replace("z", ""))] = WIRES.get(key)
    }
}

const result = parseInt(Z_WIRES.reverse().join(""), "2")
console.log({ result })

/**
 * @param {string} src1 
 * @param {string} oper 
 * @param {string} src2 
 * @param {string} dst 
 * @returns {boolean} flag indicating if specified that operation was done
 */
function applyCommand(src1, oper, src2, dst) {
    let result = 0
    if (WIRES.has(src1) && WIRES.has(src2)) {
        switch (oper) {
            case (XOR): {
                result = WIRES.get(src1) ^ WIRES.get(src2)
                break;
            }
            case (OR): {
                result = WIRES.get(src1) | WIRES.get(src2)
                break;
            }
            case (AND): {
                result = WIRES.get(src1) & WIRES.get(src2)
                break;
            }
        }
        WIRES.set(dst, result)
        return true
    }
    return false
}
