const num = 2

switch (num) {
    case (() => 1)(): {
        console.log("1")
        break
    }
    case 2: {
        console.log("2")
    }
    case 3: {
        console.log(3)
        break
    }
}

