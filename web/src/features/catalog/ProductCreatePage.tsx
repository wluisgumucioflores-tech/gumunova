import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { ArrowLeftIcon } from '../../components/icons'
import { messageForApiError } from '../../api/client'
import { createProduct, uploadProductImage, type ProductInput } from '../../api/products'
import { es } from '../../i18n/es'
import { ImagePicker } from './ImagePicker'
import { ProductForm } from './ProductForm'

export function ProductCreatePage() {
  const navigate = useNavigate()
  const [image, setImage] = useState<File | null>(null)

  async function handleSubmit(input: ProductInput) {
    const created = await createProduct(input)

    // The product exists from here on. If the image fails, the user is told and can add it
    // by editing the product; creating it again would only hit a duplicated SKU.
    let notice: { text: string; tone: 'success' | 'warning' } = { text: es.form.createdNotice, tone: 'success' }
    if (image) {
      try {
        await uploadProductImage(created.id, image)
      } catch (error) {
        notice = { text: es.form.createdImageFailed(messageForApiError(error)), tone: 'warning' }
      }
    }
    navigate('/catalogo', { state: notice })
  }

  return (
    <>
      <Link to="/catalogo" className="back-link">
        <ArrowLeftIcon />
        {es.common.back}
      </Link>
      <ProductForm
        title={es.form.createTitle}
        onSubmit={handleSubmit}
        onCancel={() => navigate('/catalogo')}
        imageSlot={<ImagePicker onChange={setImage} />}
      />
    </>
  )
}
