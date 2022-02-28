import {CrdtController} from '/js/crdt_controller.js'

export class SitesController {
    constructor(container) {
        this.container = container

        this.crdts = []
    }

    newCrdt() {
        let crdt = new CrdtController(this)
        this.crdts.push(crdt)
        this.container.append(crdt.render())
        return crdt
    }
}

