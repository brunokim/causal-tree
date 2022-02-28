import {SitesController} from '/js/sites_controller.js'

// TODO: load server state on reloads.
let controller = new SitesController($("#crdts"))
controller.newCrdt()

