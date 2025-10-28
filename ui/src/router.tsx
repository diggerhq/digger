import { createRouter } from '@tanstack/react-router';
import { routeTree } from './routeTree.gen';
import { terraformRoute } from '@/routes/manual/terraformWellKnown';


const existingChildren = (routeTree as any).children ?? [] // internal but fine

const mixedTree = routeTree.addChildren([
  ...existingChildren,   // keep all file-based routes
  terraformRoute,        // add your manual route
])  

export function getRouter() {

  return createRouter({
    routeTree: mixedTree,
    scrollRestoration: true,
  });
}
