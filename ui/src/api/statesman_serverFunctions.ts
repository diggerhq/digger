import { createServerFn } from "@tanstack/react-start"
import { createUnit, getUnit, listUnits, getUnitVersions, unlockUnit, lockUnit, getUnitStatus, deleteUnit, downloadLatestState, forcePushState, restoreUnitStateVersion } from "./statesman_units"

export const listUnitsFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string}) => data)
  .handler(async ({ data }) => {
    const startList = Date.now();
    const units : any = await listUnits(data.organisationId, data.userId, data.email);
    const listTime = Date.now() - startList;
    
    if (listTime > 3000) {
      console.log(`🔥 VERY SLOW listUnits: took ${listTime}ms (returned ${units.units?.length || 0} units)`);
    } else if (listTime > 1000) {
      console.log(`⚠️  SLOW listUnits: took ${listTime}ms (returned ${units.units?.length || 0} units)`);
    } else if (listTime > 500) {
      console.log(`⏱️  listUnits: took ${listTime}ms (returned ${units.units?.length || 0} units)`);
    }
    
    return units;
})

export const getUnitFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const unit : any = await getUnit(data.organisationId, data.userId, data.email, data.unitId)
    return unit
})

export const getUnitVersionsFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const unitVersions : any = await getUnitVersions(data.organisationId, data.userId, data.email, data.unitId)
    return unitVersions
})

export const lockUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const unit : any = await lockUnit(data.organisationId, data.userId, data.email, data.unitId)
    return unit
})

export const unlockUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const unit : any = await unlockUnit(data.organisationId, data.userId, data.email, data.unitId)
    return unit
})

export const downloadLatestStateFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const state : any = await downloadLatestState(data.organisationId, data.userId, data.email, data.unitId)
    return state
})

export const forcePushStateFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string, state: string}) => data)
  .handler(async ({ data }) => {
    const state : any = await forcePushState(data.organisationId, data.userId, data.email, data.unitId, data.state)
    return state
})

export const restoreUnitStateVersionFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string, timestamp: string, lockId: string}) => data)
  .handler(async ({ data }) => {
    const state : any = await restoreUnitStateVersion(data.organisationId, data.userId, data.email, data.unitId, data.timestamp, data.lockId)
    return state
})

export const getUnitStatusFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const unitStatus : any = await getUnitStatus(data.organisationId, data.userId, data.email, data.unitId)
    return unitStatus
})

export const createUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, name: string}) => data)
  .handler(async ({ data }) => {
    const startCreate = Date.now();
    console.log(`🔵 Starting unit creation: "${data.name}" for org ${data.organisationId}`);
    
    const unit : any = await createUnit(data.organisationId, data.userId, data.email, data.name);
    
    const createTime = Date.now() - startCreate;
    if (createTime > 3000) {
      console.log(`🔥 VERY SLOW unit creation: "${data.name}" took ${createTime}ms`);
    } else if (createTime > 1000) {
      console.log(`⚠️  SLOW unit creation: "${data.name}" took ${createTime}ms`);
    } else {
      console.log(`✅ Unit created: "${data.name}" in ${createTime}ms`);
    }
    
    return unit;
})

export const deleteUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    await deleteUnit(data.organisationId, data.userId, data.email, data.unitId)
})