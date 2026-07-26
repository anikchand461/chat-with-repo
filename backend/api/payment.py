from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from backend.auth import get_current_user
from backend.database import User
from backend.database.session import get_db
from backend.services.dodo import dodo

router = APIRouter(
    prefix="/payment",
    tags=["payment"],
)


@router.post("/checkout")
def create_checkout(
    user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):

    try:
        checkout = dodo.create_checkout(
            email=user.email,
            user_id=user.id,
        )

        return checkout

    except Exception as e:
        raise HTTPException(
            status_code=500,
            detail=str(e),
        )


@router.get("/status")
def payment_status(
    user: User = Depends(get_current_user),
):
    return {
        "plan": user.plan,
        "is_pro": user.plan == "PRO",
    }


@router.post("/webhook")
async def dodo_webhook():

    # We'll implement webhook verification later

    return {
        "received": True
    }