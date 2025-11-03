import { createServerFn } from "@tanstack/react-start"
import { createUnit, getUnit, listUnits, getUnitVersions, unlockUnit, lockUnit, getUnitStatus, deleteUnit, downloadLatestState, forcePushState, restoreUnitStateVersion } from "./statesman_units"
import { timeAsync } from "@/lib/perf.server"

export const listUnitsFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string}) => data)
  .handler(async ({ data }) => {
    const units : any = await timeAsync(
      `listUnits(org=${data.organisationId})`,
      () => listUnits(data.organisationId, data.userId, data.email)
    )
    return units
})

export const getUnitFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const unit : any = await timeAsync(
      `getUnit(${data.unitId})`,
      () => getUnit(data.organisationId, data.userId, data.email, data.unitId)
    )
    return unit
})

export const getUnitVersionsFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const unitVersions : any = await timeAsync(
      `getUnitVersions(${data.unitId})`,
      () => getUnitVersions(data.organisationId, data.userId, data.email, data.unitId)
    )
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
    const unitStatus : any = await timeAsync(
      `getUnitStatus(${data.unitId})`,
      () => getUnitStatus(data.organisationId, data.userId, data.email, data.unitId)
    )
    return unitStatus
})

export const createUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, name: string}) => data)
  .handler(async ({ data }) => {
    const unit : any = await createUnit(data.organisationId, data.userId, data.email, data.name)
    return unit
})

export const deleteUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    await deleteUnit(data.organisationId, data.userId, data.email, data.unitId)
})