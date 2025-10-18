import { createServerFn } from "@tanstack/react-start"
import { createUnit, getUnit, listUnits } from "./statesman_units"

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

export const createUnitFn = createServerFn({method: 'POST'})
  .inputValidator((data : {userId: string, organisationId: string, email: string, unitId: string}) => data)
  .handler(async ({ data }) => {
    const unit : any = await createUnit(data.organisationId, data.userId, data.email, data.unitId)
    return unit
})