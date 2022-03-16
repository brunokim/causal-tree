import { SitesController } from "./sites_controller.js";

let controller = new SitesController($("#crdts"));

fetch("/load", {
  method: "POST",
  headers: { Accept: "application/json" },
})
  .then((response) => response.json())
  .then((json) => controller.handleLoadResponse(json))
  .catch((err) => console.log(err));
