import {CrdtController} from '/js/crdt_controller.js'

export class SitesController {
    constructor(container) {
        this.container = container

        this.crdts = []
        this.graph = {}
    }

    newCrdt() {
        let crdt = new CrdtController(this)
        this.crdts.push(crdt)
        this.graph[crdt.id] = {inc: new Set(), out: new Set()}

        this.container.append(crdt.render())
        this.crdts.forEach(child => child.renderSyncArea())
        return crdt
    }

    makeCheckbox(action, localId, remoteId, isDisabled, isChecked) {
        return $("<input>", {type: "checkbox"})
            .attr("disabled", isDisabled)
            .attr("data-local", localId)
            .attr("data-remote", remoteId)
            .prop("checked", isChecked)
            .on("change", evt => this.handleCheckboxChange(action, localId, remoteId, evt.target.checked))
    }

    handleCheckboxChange(action, localId, remoteId, isChecked) {
        let source = (action == 'send' ? localId : remoteId) 
        let dest = (action == 'send' ? remoteId : localId) 
        let oppositeAction = (action == 'send' ? 'recv' : 'send') 

        if (isChecked) {
            this.connect(source, dest)
        } else {
            this.disconnect(source, dest)
        }

        // Set the corresponding checkbox in remote.
        let remoteCheckbox = $(`tr[data-action='${oppositeAction}'] input[data-local='${remoteId}'][data-remote='${localId}']`, this.container)
        remoteCheckbox.prop("checked", isChecked)
    }

    connect(source, dest) {
        this.graph[source].out.add(dest)
        this.graph[dest].inc.add(source)
    }

    disconnect(source, dest) {
        this.graph[source].out.delete(dest)
        this.graph[dest].inc.delete(source)
    }

    renderSyncArea(child) {
        let view = $("<div>")
            .addClass("sync")
            .append($(`
                <table>
                    <tr data-action="send"><th>Send</th></tr>
                    <tr data-action="recv"><th>Recv</th></tr>
                </table>
            `))
            .append($("<button>")
                .text("Sync")
                .click(evt => child.sync(evt)))

        for (let crdt of this.crdts) {
            let isSelf = false, isOut = false, isInc = false

            if (crdt === child) {
                isSelf = true
            } else {
                isOut = this.graph[child.id].out.has(crdt.id)
                isInc = this.graph[child.id].inc.has(crdt.id)
            }
            let [sendRow, recvRow] = $("tr", view).toArray()
            let checkSend = this.makeCheckbox('send', child.id, crdt.id, isSelf, isOut)
            let checkRecv = this.makeCheckbox('recv', child.id, crdt.id, isSelf, isInc)
            $(sendRow).append($("<td>").append(checkSend))
            $(recvRow).append($("<td>").append(checkRecv))
        }

        return view
    }
}

