import { createServerFn } from "@tanstack/react-start"
import { createUnit, getUnit, listUnits, getUnitVersions, unlockUnit, lockUnit, getUnitStatus, deleteUnit } from "./statesman_units"

export const listUnitsFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string}) => data)
  .handler(async ({ data }) => {
    const units : any = await listUnits(data.organisationId, data.userId, data.email)
    return units
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

export const getUnitStatusFn = createServerFn({method: 'GET'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const unitStatus : any = await getUnitStatus(data.organisationId, data.userId, data.email, data.unitId)
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