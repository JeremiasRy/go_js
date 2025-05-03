function handleButtonClick(id) {
    clearTimeout(interval)
    if (id === "white") {
        switchAttributes(white, black)
        interval = setInterval(blackCallback, 100)
        return
    }

    if (id === "black") {
        switchAttributes(black, white)
        interval = setInterval(whiteCallback, 100)
        return
    }
}

/**
 * Switch-a-roo!
 * @param {HTMLElement} a Node to disable
 * @param {HTMLElement} b Node to activate
 */
function switchAttributes(a, b) {
    a.setAttribute("disabled", true)
    a.classList.remove("active")

    b.removeAttribute("disabled")
    b.classList.add("active")
}
/**
 * Add leading zero if needed '8' => '08'
 * @param {number} time 
 * @returns 
 */
function formatTimeUnit(time) {
    return time < 10 ? "0" + time : time
}
/**
 * Format millisseconds to 00:00.0 format
 *
 * @param {number} ms Milliseconds
 * @returns {string} 
 */
function timeToDisplayForm(ms) {
    const minutes = Math.floor(ms / (60 * 1000))
    const seconds = Math.floor(ms % (60 * 1000) / 1000)
    const tenths = ms % (60 * 1000) % 1000 / 100

    return `${formatTimeUnit(minutes)}:${formatTimeUnit(seconds)}.${tenths}`
}

function startTimer(time) {
    white.addEventListener("click", () => { handleButtonClick("white") })
    black.addEventListener("click", () => { handleButtonClick("black") })

    whiteTime = time
    blackTime = time

    whiteDisplay.innerHTML = timeToDisplayForm(whiteTime)
    blackDisplay.innerHTML = timeToDisplayForm(blackTime)
}

function startGame(event) {
    event.preventDefault()
    const form = new FormData(event.target)
    const time = parseInt(form.get("time")) * 60 * 1000
    startTimer(time)

    const wrapper = document.getElementById("start_wrapper")
    wrapper.style.display = "none"

    let url = new URL(window.location.href);
    let loser = url.searchParams.get("out_of_time")
    if (loser) {
        document.getElementById("result").remove()
        url.searchParams.delete("out_of_time")
        window.history.pushState({}, '', url);
    }
    interval = setInterval(whiteCallback, 100)
}

function cancelGame() {
    clearInterval(interval)
    const wrapper = document.getElementById("start_wrapper")
    wrapper.style.display = "flex"

    let url = new URL(window.location.href);
    let loser = url.searchParams.get("out_of_time")
    if (loser) {
        const p = document.createElement("p")
        p.id = "result"
        p.innerHTML = `${loser} run out of time!`
        wrapper.prepend(p)
    }
    reset()
}

function reset() {
    white.classList.add("active")
    white.removeAttribute("disabled")
    black.classList.remove("active")
    black.setAttribute("disabled", true)
}

function whiteCallback() {
    whiteTime -= 100
    if (whiteTime < 0) {
        outOfTime("White")
        return
    }
    whiteDisplay.innerHTML = timeToDisplayForm(whiteTime)
}
function blackCallback() {
    blackTime -= 100
    if (blackTime < 0) {
        outOfTime("Black")
        return
    }
    blackDisplay.innerHTML = timeToDisplayForm(blackTime)
}

function outOfTime(loser) {
    let url = new URL(window.location.href);
    url.searchParams.set('out_of_time', loser);
    window.history.pushState({}, '', url);
    cancelGame()
}

let interval;
let whiteTime;
let blackTime;
const white = document.getElementById("white")
const black = document.getElementById("black")
const whiteDisplay = document.getElementById("white_time")
const blackDisplay = document.getElementById("black_time")

document.getElementById("back_button").addEventListener("click", cancelGame)