import childProcess from 'node:child_process';
import { syncBuiltinESMExports } from 'node:module';

let installed = false;

// The host owns the worker's process group, including Chromium descendants.
// Playwright normally detaches browsers; retaining this group lets the host
// terminate them even if the JavaScript worker becomes unresponsive.
export function retainProcessGroup() {
  if (installed) return;
  installed = true;
  const spawn = childProcess.spawn;
  childProcess.spawn = function(command, args, options) {
    if (!Array.isArray(args)) return spawn.call(this, command, { ...args, detached: false });
    return spawn.call(this, command, args, { ...options, detached: false });
  };
  syncBuiltinESMExports();
}
