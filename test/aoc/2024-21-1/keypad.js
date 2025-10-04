import { up, down, left, right } from "./direction"

const EMPTY_PAD = "EMPTY"
const ACTION_BUTTON = "A"

const BUTTON_ZERO = "0"
const BUTTON_ONE = "1"
const BUTTON_TWO = "2"
const BUTTON_THREE = "3"
const BUTTON_FOUR = "4"
const BUTTON_FIVE = "5"
const BUTTON_SIX = "6"
const BUTTON_SEVEN = "7"
const BUTTON_EIGHT = "8"
const BUTTON_NINE = "9"

const BUTTON_LEFT = "<"
const BUTTON_RIGHT = ">"
const BUTTON_DOWN = "v"
const BUTTON_UP = "^"
const NUMERIC = [BUTTON_SEVEN, BUTTON_EIGHT, BUTTON_NINE, BUTTON_FOUR, BUTTON_FIVE, BUTTON_SIX, BUTTON_ONE, BUTTON_TWO, BUTTON_THREE, EMPTY_PAD, BUTTON_ZERO, ACTION_BUTTON]
const NUMERIC_START = NUMERIC.indexOf(ACTION_BUTTON)

const DIRECTIONAL = [EMPTY_PAD, BUTTON_UP, ACTION_BUTTON, BUTTON_LEFT, BUTTON_DOWN, BUTTON_RIGHT]
const DIRECTIONAL_START = DIRECTIONAL.indexOf(ACTION_BUTTON)

const DIR_BUTTON_TO_OUTPUT = {
    BUTTON_LEFT: left,
    BUTTON_DOWN: down,
    BUTTON_UP: up,
    BUTTON_RIGHT: right
}


class NumericPad {
    current = NUMERIC_START

    move(input) {
        let move = DIR_BUTTON_TO_OUTPUT[input]

        if (!move) {
            throw new Error(`did not find output for input: ${input}`)
        }

        this.current += move

        if (NUMERIC[this.current] === EMPTY_PAD) {
            throw new Error(`landed on an empty pad input: ${input} current: ${this.current}`)
        }

        return null
    }

    action() {
        return NUMERIC[this.current]
    }
}

class DirectionalPad {
    current = DIRECTIONAL_START

    move(input) {
        let move = DIR_BUTTON_TO_OUTPUT[input]

        if (!move) {
            throw new Error(`did not find output for input: ${input}`)
        }

        this.current += move

        if (DIRECTIONAL[this.current] === EMPTY_PAD) {
            throw new Error(`landed on an empty pad input: ${input} current: ${this.current}`)
        }

        return null
    }

    action() {
        return DIRECTIONAL[this.current]
    }
}



