
export class Crdt {
    constructor(states) {
        this.states = states;
        this.time = 0;

        this.keyListener = this.handleKeypress.bind(this)
        window.addEventListener("keydown", this.keyListener)
    }
    
    destroy() {
        window.removeEventListener("keydown", this.keyListener)
    }

    handleKeypress(evt) {
        if (evt.defaultPrevented) {
            return
        }

        let handled = false
        switch(evt.key) {
        case "ArrowLeft":
            this.addTime(-1)
            handled = true
            break
        case "ArrowRight":
            this.addTime(+1)
            handled = true
            break
        }

        if (handled) {
            evt.preventDefault()
        }
    }

    render() {
        $("#crdt")
            .html("")
            .append(this.controls())
            .append($("<div>")
                .addClass("state")
                .html(JSON.stringify(this.state())))
    }

    controls() {
        return $("<div>")
            .addClass("controls")
            .append($("<button>").text("-100").click(() => this.addTime(-100)))
            .append($("<button>").text("-10").click(() => this.addTime(-10)))
            .append($("<button>").text("-1").click(() => this.addTime(-1)))
            .append($("<input>")
                .attr("type", "number")
                .val(this.time)
                .change((evt) => this.goToTime(evt.target.valueAsNumber)))
            .append($("<button>").text("+1").click(() => this.addTime(+1)))
            .append($("<button>").text("+10").click(() => this.addTime(+10)))
            .append($("<button>").text("+100").click(() => this.addTime(+100)))
    }

    addTime(dt) {
        this.goToTime(this.time+dt)
    }

    goToTime(t) {
        if (t < 0) {
            t = 0
        }
        if (t > this.states.length-1) {
            t = this.states.length-1
        }
        if (t != this.time) {
            this.time = t
            this.render()
        }
    }

    state(time) {
        if (time === undefined) {
            time = this.time
        }
        return this.states[time]
    }
}

