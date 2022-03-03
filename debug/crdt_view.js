
// Connect each list partial state to its request.
function prepareStates(log) {
    let states = []
    let requests = {
        'edit': [],
        'fork': [],
        'sync': [],
    }
    for (let record of log) {
        let recordType = record['Type']
        switch(recordType) {
            case "edit":
            case "fork":
            case "sync":
                requests[recordType].push(record['Request'])
                break
            case "editStep":
            case "forkStep":
            case "syncStep":
                let requestType = recordType.slice(0, -4) // Remove 'Step' from type name.
                let requestIndex = record['ReqIdx']
                record['Request'] = requests[requestType][requestIndex]
                // fallthrough
            case "test":
                states.push(record)
                break
            default:
                throw new Error(`Unknown record type ${recordType}`)
        }
    }
    return states
}

export class Crdt {
    constructor(log) {
        this.states = prepareStates(log)
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
        let state = this.state()
        $("#crdt")
            .html("")
            .append(this.controls())
            .append($("<div>")
                .addClass("state")
                .append($("<h2>").append(this.renderTitle(state)))
                .append(this.renderSites(state['Sites'])))
    }

    renderTitle(state) {
        switch(state['Type']) {
            case 'test':
                return state['Action']
            case 'editStep':
                let stepIndex = state['StepIdx']
                let op = state['Request'].ops[stepIndex]
                return `Edit request #${state['ReqIdx']} @ ${stepIndex} - ${op.op} ${op.ch} at list #${state['LocalIdx']}`
            case 'forkStep':
                return `Fork list #${state['LocalIdx']} into list #${state['RemoteIdx']}`
            case 'syncStep':
                return `Merge list #${state['LocalIdx']} from list #${state['RemoteIdx']}`
        }
        return ''
    }

    renderSites(sites) {
        let siteEls = []
        let i = 0
        for (let site of sites) {
            siteEls.push(this.renderSite(site, i))
            i++
        }
        return siteEls
    }

    renderSite(site, i) {
        let index = site['Sitemap'].indexOf(site['SiteID']);
        return $("<div>")
            .addClass("site")
            .append($("<h3>").append(`List #${i}`))
            .append($("<h4>").append("Sitemap"))
            .append($("<ol>")
                .addClass("sitemap")
                .attr("start", 0)
                .append(this.renderSiteIDs(site['Sitemap'], index)))
            .append($("<h4>").append("Weave"))
            .append($("<div>")
                .addClass("weave")
                .append(this.renderAtoms(site['Weave'], site['Cursor'])))
    }

    renderSiteIDs(sitemap, index) {
        let ids = []
        let i = 0
        for (let siteID of sitemap) {
            let el = $("<li>").append(siteID)
            if (i == index) {
                el.addClass("highlight")
            }
            ids.push(el)
            i++
        }
        return ids
    }

    renderAtoms(weave, cursor) {
        let atoms = []
        for (let atom of weave) {
            atoms.push(this.renderAtom(atom, cursor))
        }
        return atoms
    }

    renderAtom(atom, cursor) {
        // TODO: replace non-printable characters in value.
        let atomEl = $("<div>")
            .addClass("atom")
            .append($("<div>")
                .addClass("atom-id")
                .text(idString(atom['ID'])))
            .append($("<div>")
                .addClass("atom-value")
                .text(atom['Value']))
            .append($("<div>")
                .addClass("atom-cause")
                .text(idString(atom['Cause'])))
        if (idString(atom['ID']) == idString(cursor)) {
            atomEl.addClass("cursor")
        }
        return atomEl
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

function idString(id) {
    return `S${id['Site']}@T${id['Timestamp']}`
}
